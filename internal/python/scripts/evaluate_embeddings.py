#!/usr/bin/env python3
"""Embedding model evaluation tool for gh-star-search.

Compares embedding models on real GitHub starred repository data using
curated test queries and standard information retrieval metrics.

Usage:
    1. Ensure database is populated:
       gh star-search sync

    2. Customize models in eval_config.toml:
       [models.current]  - baseline model (e5-small-v2)
       [models.candidate] - alternative model to compare

    3. Run evaluation:
       cd internal/python/scripts
       uv run python evaluate_embeddings.py

    4. Review outputs:
       - Terminal: Rich formatted comparison table
       - eval_results/evaluation_YYYYMMDD.json: Raw data (gitignored)
       - eval_results/evaluation_YYYYMMDD.md: Summary report (tracked)

Evaluation metrics:
    - MRR (Mean Reciprocal Rank): Position of first relevant result
    - Precision@k: Fraction of relevant results in top k
    - Recall@k: Fraction of relevant results found in top k
    - NDCG@k: Normalized Discounted Cumulative Gain (graded relevance)

The tool uses 30 curated test queries covering technology-focused,
use-case-focused, composite, and edge-case searches. Statistical
significance is tested using paired t-test on per-query MRR values.
"""

import argparse
import contextlib
import json
import math
import shutil
import tomllib
from collections.abc import Iterator
from dataclasses import asdict, dataclass
from datetime import datetime
from pathlib import Path
from statistics import mean
from textwrap import dedent

import duckdb
from embedding_cache import EmbeddingCache, _validate_identifier, sync_model_embeddings
from rich.console import Console
from rich.table import Table
from sentence_transformers import SentenceTransformer

DEFAULT_DB_PATH = str(Path.home() / ".local/share/gh-star-search/stars.db")
DEFAULT_STAR_BOOST_COEFFICIENT = 0.1


@dataclass(frozen=True)
class ModelConfig:
    """Configuration for an embedding model."""

    name: str
    model_id: str
    dimensions: int


@dataclass(frozen=True)
class Query:
    """Test query with expected results."""

    id: str
    query: str
    category: str
    expected_repos: list[str]
    relevance_grades: dict[str, int]


@dataclass
class QueryMetrics:
    """Metrics for a single query evaluation."""

    mrr: float
    precision_at_5: float
    precision_at_10: float
    recall_at_10: float
    recall_at_20: float
    ndcg_at_10: float


@dataclass
class QueryResult:
    """Results for a single query evaluation."""

    query_id: str
    query: str
    category: str
    metrics: QueryMetrics
    top_5_results: list[str]


@dataclass
class ModelEvaluation:
    """Complete evaluation results for one model."""

    model_name: str
    model_id: str
    dimensions: int
    aggregate_metrics: QueryMetrics
    per_query_results: list[QueryResult]


@dataclass
class ModelComparison:
    """Statistical comparison between two models."""

    model_a: str
    model_b: str
    metric_deltas: dict[str, float]
    statistical_test: dict[str, float]
    recommendation: str


def _chunks(items: list, size: int) -> Iterator[list]:
    """Split list into chunks of specified size."""
    for i in range(0, len(items), size):
        yield items[i : i + size]


def _find_uv() -> str:
    """Find uv executable path."""
    if path := shutil.which("uv"):
        return path
    raise RuntimeError("uv not found in PATH")


def _build_embedding_input(repo: dict) -> str:
    """Build embedding input text from repo metadata.

    Matches buildEmbeddingInput() in cmd/sync_embed.go.
    """
    parts = [repo.get("full_name", "")]

    if repo.get("purpose"):
        parts.append(repo["purpose"])

    if repo.get("description"):
        parts.append(repo["description"])

    if repo.get("topics_array"):
        topics_raw = repo["topics_array"]
        topics = json.loads(topics_raw) if isinstance(topics_raw, str) else topics_raw

        if topics:
            parts.append(" ".join(topics))

    return ". ".join(parts)


def _call_embed_script(
    texts: list[str], model_id: str, uv_path: str, project_dir: Path
) -> list[list[float]]:
    """Call embed.py subprocess to generate embeddings."""
    import subprocess

    cmd = [
        uv_path,
        "run",
        "--project",
        str(project_dir),
        "--quiet",
        "python",
        str(project_dir / "embed.py"),
        "--model",
        model_id,
        "--stdin",
    ]

    proc = subprocess.run(
        cmd,
        input=json.dumps(texts).encode(),
        capture_output=True,
        timeout=120,
        cwd=project_dir,
        check=False,
    )

    if proc.returncode != 0:
        raise RuntimeError(f"embed.py failed: {proc.stderr.decode()}")

    result = json.loads(proc.stdout.decode())
    return result["embeddings"]


def _apply_ranking_boosts(
    results: list[dict], star_boost_coefficient: float = DEFAULT_STAR_BOOST_COEFFICIENT
) -> list[dict]:
    """Apply star boost and recency factor to scores.

    Matches applyRankingBoosts() in internal/query/engine.go.
    """
    for result in results:
        base_score = result["base_score"]

        star_boost = 1.0 + (
            star_boost_coefficient * math.log10(result["stargazers_count"] + 1) / 6.0
        )

        updated_at = result["updated_at"]
        if isinstance(updated_at, str):
            updated_at = datetime.fromisoformat(updated_at.replace("Z", "+00:00"))
        days_since_update = (datetime.now(updated_at.tzinfo) - updated_at).days
        recency_factor = 1.0 - 0.2 * min(1.0, days_since_update / 365.0)

        result["final_score"] = base_score * star_boost * recency_factor

    results.sort(key=lambda r: r["final_score"], reverse=True)
    return results


def _compute_mrr(results: list[dict], relevant_repos: list[str]) -> float:
    """Compute Mean Reciprocal Rank."""
    for rank, result in enumerate(results, start=1):
        if result["full_name"] in relevant_repos:
            return 1.0 / rank
    return 0.0


def _compute_precision_at_k(
    results: list[dict], relevant_repos: list[str], k: int
) -> float:
    """Compute Precision@k."""
    top_k = results[:k]
    relevant_count = sum(1 for r in top_k if r["full_name"] in relevant_repos)
    return relevant_count / k if k > 0 else 0.0


def _compute_recall_at_k(
    results: list[dict], relevant_repos: list[str], k: int
) -> float:
    """Compute Recall@k."""
    if not relevant_repos:
        return 0.0
    top_k = results[:k]
    found = sum(1 for r in top_k if r["full_name"] in relevant_repos)
    return found / len(relevant_repos)


def _compute_ndcg_at_k(
    results: list[dict], relevance_grades: dict[str, int], k: int
) -> float:
    """Compute Normalized Discounted Cumulative Gain@k."""
    dcg = sum(
        relevance_grades.get(r["full_name"], 0) / math.log2(i + 2)
        for i, r in enumerate(results[:k])
    )

    ideal_rels = sorted(relevance_grades.values(), reverse=True)[:k]
    idcg = (
        sum(rel / math.log2(i + 2) for i, rel in enumerate(ideal_rels))
        if ideal_rels
        else 1.0
    )

    return dcg / idcg if idcg > 0 else 0.0


class EmbeddingEvaluator:
    """Evaluator for comparing embedding models."""

    def __init__(
        self,
        db_path: str,
        config_path: str,
        use_cache: bool = True,
        star_boost_coefficient: float = DEFAULT_STAR_BOOST_COEFFICIENT,
    ) -> None:
        """Initialize evaluator.

        Args:
            db_path: Path to main database
            config_path: Path to TOML configuration file
            use_cache: Whether to use persistent embedding cache
            star_boost_coefficient: Star boost coefficient for ranking
        """
        self.db = duckdb.connect(db_path)
        self.db_path = db_path
        self.config = self._load_config(config_path)
        self.uv_path = _find_uv()
        self.project_dir = Path(__file__).parent
        self.use_cache = use_cache
        self.cache = EmbeddingCache() if use_cache else None
        self.star_boost_coefficient = star_boost_coefficient
        self._loaded_models: dict[str, SentenceTransformer] = {}

    def _load_config(self, config_path: str) -> dict:
        """Load configuration from TOML file."""
        with open(config_path, "rb") as f:
            return tomllib.load(f)

    def _setup_eval_table(self, model_name: str, dimensions: int) -> str:
        """Create temporary table for model embeddings."""
        table_name = f"eval_embeddings_{model_name.replace('-', '_')}"
        _validate_identifier(table_name)

        self.db.execute(f"DROP TABLE IF EXISTS {table_name}")
        self.db.execute(
            dedent(f"""\
            CREATE TABLE {table_name} (
                repo_id VARCHAR,
                embedding JSON,
                FOREIGN KEY (repo_id) REFERENCES repositories(id)
            )""")
        )

        return table_name

    def _get_or_load_model(self, model_id: str) -> SentenceTransformer:
        """Get model from cache or load it."""
        if model_id not in self._loaded_models:
            print(f"  Loading model: {model_id}")
            self._loaded_models[model_id] = SentenceTransformer(model_id)
        return self._loaded_models[model_id]

    def _generate_embeddings_in_process(
        self, texts: list[str], model_id: str
    ) -> list[list[float]]:
        """Generate embeddings in-process (no subprocess overhead)."""
        model = self._get_or_load_model(model_id)
        embeddings = model.encode(texts, show_progress_bar=False)
        return embeddings.tolist()

    def _sync_embeddings_with_cache(self, model_config: ModelConfig) -> str:
        """Sync embeddings using persistent cache (incremental)."""
        print("  Syncing embeddings with cache...")

        def embedding_generator(texts: list[str]) -> list[list[float]]:
            return self._generate_embeddings_in_process(texts, model_config.model_id)

        embedded_count = sync_model_embeddings(
            self.db_path,
            model_config.model_id,
            model_config.dimensions,
            embedding_generator,
            batch_size=128,
        )

        if embedded_count > 0:
            print(f"  Embedded {embedded_count} new/changed repos")
        else:
            print("  All repos already cached")

        table_name = f"eval_embeddings_{model_config.name.replace('-', '_')}"
        self.cache.create_eval_view(self.db, model_config.model_id, table_name)

        return table_name

    def _generate_embeddings_for_model(self, model_config: ModelConfig) -> str:
        """Generate embeddings for all repos using specified model."""
        print("  Fetching repositories...")
        repos = self.db.execute(
            dedent("""\
            SELECT id, full_name, description, purpose, topics_array
            FROM repositories
            ORDER BY id""")
        ).fetchall()

        print(f"  Found {len(repos)} repositories")

        table_name = self._setup_eval_table(model_config.name, model_config.dimensions)

        batch_size = 32
        total_batches = (len(repos) + batch_size - 1) // batch_size

        print(f"  Generating embeddings in {total_batches} batches...")

        for batch_idx, batch in enumerate(_chunks(repos, batch_size), start=1):
            print(f"    Batch {batch_idx}/{total_batches}", end="\r")

            texts = [
                _build_embedding_input(
                    {
                        "full_name": r[1],
                        "description": r[2],
                        "purpose": r[3],
                        "topics_array": r[4],
                    }
                )
                for r in batch
            ]

            embeddings = _call_embed_script(
                texts, model_config.model_id, self.uv_path, self.project_dir
            )

            for repo, emb in zip(batch, embeddings, strict=True):
                self.db.execute(
                    f"INSERT INTO {table_name} (repo_id, embedding) "
                    "VALUES (?, ?::JSON)",
                    [repo[0], json.dumps(emb)],
                )

        print("\n  Embeddings generated successfully")
        return table_name

    def _run_query(
        self,
        query_text: str,
        model_config: ModelConfig,
        table_name: str,
        limit: int = 50,
    ) -> list[dict]:
        """Execute vector search query using model embeddings."""
        if self.use_cache:
            query_emb = self._generate_embeddings_in_process(
                [query_text], model_config.model_id
            )[0]
        else:
            query_emb = _call_embed_script(
                [query_text], model_config.model_id, self.uv_path, self.project_dir
            )[0]

        results = self.db.execute(
            dedent(f"""\
            SELECT
                r.id,
                r.full_name,
                r.description,
                r.stargazers_count,
                r.updated_at,
                array_cosine_similarity(
                    CAST(e.embedding AS FLOAT[{model_config.dimensions}]),
                    ?::FLOAT[{model_config.dimensions}]
                ) AS base_score
            FROM repositories r
            JOIN {table_name} e ON r.id = e.repo_id
            WHERE e.embedding IS NOT NULL
            ORDER BY base_score DESC
            LIMIT ?"""),
            [query_emb, limit],
        ).fetchall()

        result_dicts = [
            {
                "id": r[0],
                "full_name": r[1],
                "description": r[2],
                "stargazers_count": r[3],
                "updated_at": r[4],
                "base_score": r[5],
            }
            for r in results
        ]

        return _apply_ranking_boosts(result_dicts, self.star_boost_coefficient)

    def _evaluate_query(
        self, query: Query, model_config: ModelConfig, table_name: str
    ) -> QueryResult:
        """Evaluate a single query."""
        results = self._run_query(query.query, model_config, table_name, limit=50)

        metrics = QueryMetrics(
            mrr=_compute_mrr(results, query.expected_repos),
            precision_at_5=_compute_precision_at_k(results, query.expected_repos, 5),
            precision_at_10=_compute_precision_at_k(results, query.expected_repos, 10),
            recall_at_10=_compute_recall_at_k(results, query.expected_repos, 10),
            recall_at_20=_compute_recall_at_k(results, query.expected_repos, 20),
            ndcg_at_10=_compute_ndcg_at_k(results, query.relevance_grades, 10),
        )

        return QueryResult(
            query_id=query.id,
            query=query.query,
            category=query.category,
            metrics=metrics,
            top_5_results=[r["full_name"] for r in results[:5]],
        )

    def evaluate_model(self, model_config: ModelConfig) -> ModelEvaluation:
        """Run full evaluation for one model."""
        print(f"\n{'=' * 60}")
        print(f"Evaluating model: {model_config.name}")
        print(f"  Model ID: {model_config.model_id}")
        print(f"  Dimensions: {model_config.dimensions}")
        print(f"{'=' * 60}\n")

        if self.use_cache:
            table_name = self._sync_embeddings_with_cache(model_config)
        else:
            table_name = self._generate_embeddings_for_model(model_config)

        queries = [
            Query(
                id=q["id"],
                query=q["query"],
                category=q["category"],
                expected_repos=q["expected_repos"],
                relevance_grades=q.get("relevance_grades", {}),
            )
            for q in self.config["queries"]
        ]

        print(f"\n  Running {len(queries)} test queries...")

        query_results = []
        for idx, query in enumerate(queries, start=1):
            print(f'    Query {idx}/{len(queries)}: "{query.query}"', end="\r")
            result = self._evaluate_query(query, model_config, table_name)
            query_results.append(result)

        print("\n  Query evaluation complete")

        aggregate = QueryMetrics(
            mrr=mean([q.metrics.mrr for q in query_results]),
            precision_at_5=mean([q.metrics.precision_at_5 for q in query_results]),
            precision_at_10=mean([q.metrics.precision_at_10 for q in query_results]),
            recall_at_10=mean([q.metrics.recall_at_10 for q in query_results]),
            recall_at_20=mean([q.metrics.recall_at_20 for q in query_results]),
            ndcg_at_10=mean([q.metrics.ndcg_at_10 for q in query_results]),
        )

        if self.use_cache:
            self.db.execute(f"DROP VIEW IF EXISTS {table_name}")
            with contextlib.suppress(Exception):
                self.db.execute(f"DETACH {table_name}_cache")
        else:
            self.db.execute(f"DROP TABLE IF EXISTS {table_name}")

        return ModelEvaluation(
            model_name=model_config.name,
            model_id=model_config.model_id,
            dimensions=model_config.dimensions,
            aggregate_metrics=aggregate,
            per_query_results=query_results,
        )

    def compare_models(
        self, model_a_results: ModelEvaluation, model_b_results: ModelEvaluation
    ) -> ModelComparison:
        """Statistical comparison between two models."""
        try:
            from scipy import stats

            mrr_a = [q.metrics.mrr for q in model_a_results.per_query_results]
            mrr_b = [q.metrics.mrr for q in model_b_results.per_query_results]

            t_stat, p_value = stats.ttest_rel(mrr_b, mrr_a)
        except ImportError:
            t_stat = 0.0
            p_value = 1.0

        deltas = {
            "mrr": model_b_results.aggregate_metrics.mrr
            - model_a_results.aggregate_metrics.mrr,
            "precision_at_5": model_b_results.aggregate_metrics.precision_at_5
            - model_a_results.aggregate_metrics.precision_at_5,
            "precision_at_10": model_b_results.aggregate_metrics.precision_at_10
            - model_a_results.aggregate_metrics.precision_at_10,
            "recall_at_10": model_b_results.aggregate_metrics.recall_at_10
            - model_a_results.aggregate_metrics.recall_at_10,
            "recall_at_20": model_b_results.aggregate_metrics.recall_at_20
            - model_a_results.aggregate_metrics.recall_at_20,
            "ndcg_at_10": model_b_results.aggregate_metrics.ndcg_at_10
            - model_a_results.aggregate_metrics.ndcg_at_10,
        }

        if p_value < 0.05 and deltas["mrr"] > 0.05:
            recommendation = (
                f"{model_b_results.model_name} significantly better (p={p_value:.4f})"
            )
        elif p_value < 0.05 and deltas["mrr"] < -0.05:
            recommendation = (
                f"{model_a_results.model_name} significantly better (p={p_value:.4f})"
            )
        else:
            recommendation = "No significant difference detected"

        return ModelComparison(
            model_a=model_a_results.model_name,
            model_b=model_b_results.model_name,
            metric_deltas=deltas,
            statistical_test={
                "method": "paired_t_test",
                "metric": "mrr",
                "t_statistic": float(t_stat),
                "p_value": float(p_value),
            },
            recommendation=recommendation,
        )

    def write_json_output(self, results: dict, timestamp: str) -> None:
        """Write complete evaluation results to JSON."""
        output_dir = Path("eval_results")
        output_file = output_dir / f"evaluation_{timestamp}.json"

        with open(output_file, "w") as f:
            json.dump(results, f, indent=2)

        print(f"\nJSON results written to: {output_file}")

    def display_results_terminal(
        self,
        model_a: ModelEvaluation,
        model_b: ModelEvaluation,
        comparison: ModelComparison,
    ) -> None:
        """Display results as rich formatted table in terminal."""
        console = Console()

        table = Table(
            title="\nModel Comparison Results", show_header=True, header_style="bold"
        )
        table.add_column("Metric", style="cyan")
        table.add_column(model_a.model_name, style="magenta")
        table.add_column(model_b.model_name, style="green")
        table.add_column("Delta", style="yellow")

        metrics = [
            ("MRR", "mrr"),
            ("Precision@5", "precision_at_5"),
            ("Precision@10", "precision_at_10"),
            ("Recall@10", "recall_at_10"),
            ("Recall@20", "recall_at_20"),
            ("NDCG@10", "ndcg_at_10"),
        ]

        for label, key in metrics:
            val_a = getattr(model_a.aggregate_metrics, key)
            val_b = getattr(model_b.aggregate_metrics, key)
            delta = val_b - val_a

            table.add_row(label, f"{val_a:.3f}", f"{val_b:.3f}", f"{delta:+.3f}")

        console.print(table)
        console.print(f"\n[bold]Recommendation:[/bold] {comparison.recommendation}")
        p_value = comparison.statistical_test["p_value"]
        method = comparison.statistical_test["method"]
        console.print(f"[dim]Statistical test: {method} (p={p_value:.4f})[/dim]\n")

    def write_markdown_report(
        self,
        model_a: ModelEvaluation,
        model_b: ModelEvaluation,
        comparison: ModelComparison,
        timestamp: str,
    ) -> None:
        """Generate markdown summary report."""
        output_dir = Path("eval_results")
        output_file = output_dir / f"evaluation_{timestamp}.md"

        md = []
        md.append("# Embedding Model Evaluation Report")
        md.append(f"\n**Date**: {timestamp}")
        md.append(
            f"\n**Models Compared**: {model_a.model_name} vs {model_b.model_name}"
        )

        md.append("\n## Summary\n")
        md.append(f"| Metric | {model_a.model_name} | {model_b.model_name} | Delta |")
        md.append("|--------|----------|----------|-------|")

        metrics = [
            ("MRR", "mrr"),
            ("Precision@5", "precision_at_5"),
            ("Precision@10", "precision_at_10"),
            ("Recall@10", "recall_at_10"),
            ("Recall@20", "recall_at_20"),
            ("NDCG@10", "ndcg_at_10"),
        ]

        for label, key in metrics:
            val_a = getattr(model_a.aggregate_metrics, key)
            val_b = getattr(model_b.aggregate_metrics, key)
            delta = val_b - val_a
            md.append(f"| {label} | {val_a:.3f} | {val_b:.3f} | {delta:+.3f} |")

        md.append("\n## Recommendation\n")
        md.append(comparison.recommendation)

        md.append("\n## Statistical Test\n")
        md.append(f"- Method: {comparison.statistical_test['method']}")
        md.append(f"- Metric: {comparison.statistical_test['metric']}")
        md.append(f"- t-statistic: {comparison.statistical_test['t_statistic']:.4f}")
        md.append(f"- p-value: {comparison.statistical_test['p_value']:.4f}")

        md.append("\n## Sample Query Results\n")
        for query_result in model_a.per_query_results[:5]:
            md.append(f'\n### Query: "{query_result.query}"')
            md.append(f"- Category: {query_result.category}")
            md.append(f"- MRR: {query_result.metrics.mrr:.3f}")
            md.append(f"- Precision@5: {query_result.metrics.precision_at_5:.3f}")
            md.append("- Top 5 Results:")
            for result in query_result.top_5_results:
                md.append(f"  - {result}")

        with open(output_file, "w") as f:
            f.write("\n".join(md))

        print(f"Markdown report written to: {output_file}")


def main() -> None:
    """Execute embedding model evaluation from command line."""
    parser = argparse.ArgumentParser(description="Evaluate embedding models")
    parser.add_argument(
        "--config", default="eval_config.toml", help="Path to configuration file"
    )
    parser.add_argument(
        "--db",
        default=DEFAULT_DB_PATH,
        help="Path to database file",
    )
    parser.add_argument(
        "--no-cache",
        action="store_true",
        help="Disable persistent embedding cache (slower, always regenerates)",
    )
    parser.add_argument(
        "--cache-stats", action="store_true", help="Show cache statistics and exit"
    )
    parser.add_argument(
        "--star-boost",
        type=float,
        default=None,
        help=(
            f"Star boost coefficient "
            f"(default: {DEFAULT_STAR_BOOST_COEFFICIENT}, or from config)"
        ),
    )
    args = parser.parse_args()

    if args.cache_stats:
        cache = EmbeddingCache()
        stats = cache.get_cache_stats()

        if not stats:
            print("No cached embeddings found")
            return

        print("\nEmbedding Cache Statistics:")
        print("=" * 70)
        for model_id, model_stats in stats.items():
            print(f"\nModel: {model_id}")
            print(f"  Embeddings: {model_stats['embeddings_count']:,}")
            print(f"  Dimensions: {model_stats['dimensions']}")
            print(f"  Last sync: {model_stats['last_sync']}")
            print(f"  DB size: {model_stats['db_size_mb']:.1f} MB")
        print()
        return

    use_cache = not args.no_cache
    if use_cache:
        print("Using persistent embedding cache (incremental updates)")
        print("Use --no-cache to disable caching\n")
    else:
        print("Cache disabled - will regenerate all embeddings\n")

    with open(args.config, "rb") as f:
        config = tomllib.load(f)

    star_boost = args.star_boost
    if star_boost is None:
        star_boost = config.get("settings", {}).get(
            "star_boost_coefficient", DEFAULT_STAR_BOOST_COEFFICIENT
        )

    print(f"Star boost coefficient: {star_boost}")

    evaluator = EmbeddingEvaluator(
        args.db, args.config, use_cache=use_cache, star_boost_coefficient=star_boost
    )

    model_current = ModelConfig(
        name=evaluator.config["models"]["current"]["name"],
        model_id=evaluator.config["models"]["current"]["model_id"],
        dimensions=evaluator.config["models"]["current"]["dimensions"],
    )

    model_candidate = ModelConfig(
        name=evaluator.config["models"]["candidate"]["name"],
        model_id=evaluator.config["models"]["candidate"]["model_id"],
        dimensions=evaluator.config["models"]["candidate"]["dimensions"],
    )

    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")

    result_current = evaluator.evaluate_model(model_current)
    result_candidate = evaluator.evaluate_model(model_candidate)

    comparison = evaluator.compare_models(result_current, result_candidate)

    complete_results = {
        "timestamp": timestamp,
        "settings": {"star_boost_coefficient": star_boost},
        "models": [asdict(result_current), asdict(result_candidate)],
        "comparison": asdict(comparison),
    }

    evaluator.write_json_output(complete_results, timestamp)
    evaluator.display_results_terminal(result_current, result_candidate, comparison)
    evaluator.write_markdown_report(
        result_current, result_candidate, comparison, timestamp
    )

    print(f"\n{'=' * 60}")
    print("Evaluation complete!")
    print(f"{'=' * 60}\n")


if __name__ == "__main__":
    main()
