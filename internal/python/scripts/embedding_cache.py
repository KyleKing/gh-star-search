#!/usr/bin/env python3
"""Model-versioned embedding cache management.

Maintains parallel databases for each embedding model, computing embeddings
only once per model and incrementally updating when repos change.
"""

import json
import re
from collections.abc import Callable
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from textwrap import dedent

import duckdb

_SAFE_IDENTIFIER = re.compile(r"^[a-zA-Z_][a-zA-Z0-9_]*$")

_REPO_COLUMNS = (
    "id",
    "full_name",
    "description",
    "purpose",
    "topics_array",
    "content_hash",
)


def _validate_identifier(name: str) -> str:
    if not _SAFE_IDENTIFIER.match(name):
        raise ValueError(f"Unsafe SQL identifier: {name!r}")
    return name


def _row_to_repo_dict(row: tuple) -> dict:
    return dict(zip(_REPO_COLUMNS, row, strict=True))


@dataclass(frozen=True)
class ModelCacheInfo:
    """Metadata for a model's embedding cache."""

    model_id: str
    db_path: Path
    dimensions: int
    total_repos: int
    last_sync: datetime
    created: datetime


class EmbeddingCache:
    """Manages model-versioned embedding caches."""

    def __init__(self, cache_dir: Path | None = None) -> None:
        """Initialize embedding cache.

        Args:
            cache_dir: Directory for cache storage
                (default: ~/.local/share/gh-star-search/embeddings)
        """
        if cache_dir is None:
            cache_dir = Path.home() / ".local/share/gh-star-search/embeddings"
        self.cache_dir = cache_dir
        self.cache_dir.mkdir(parents=True, exist_ok=True)
        self.metadata_file = cache_dir / "metadata.json"

    def _sanitize_model_id(self, model_id: str) -> str:
        """Convert model ID to safe filename."""
        return model_id.replace("/", "__").replace(":", "_").replace(".", "_")

    def _get_cache_db_path(self, model_id: str) -> Path:
        """Get path to cache database for model."""
        return self.cache_dir / f"{self._sanitize_model_id(model_id)}.db"

    def _load_metadata(self) -> dict:
        """Load cache metadata registry."""
        if not self.metadata_file.exists():
            return {"models": {}}
        with open(self.metadata_file) as f:
            return json.load(f)

    def _save_metadata(self, metadata: dict) -> None:
        """Save cache metadata registry."""
        with open(self.metadata_file, "w") as f:
            json.dump(metadata, f, indent=2, default=str)

    def get_cache_info(self, model_id: str) -> ModelCacheInfo | None:
        """Get cache info for a model, or None if not cached."""
        metadata = self._load_metadata()
        if model_id not in metadata["models"]:
            return None

        model_meta = metadata["models"][model_id]
        return ModelCacheInfo(
            model_id=model_id,
            db_path=self.cache_dir / model_meta["db_path"],
            dimensions=model_meta["dimensions"],
            total_repos=model_meta["total_repos"],
            last_sync=datetime.fromisoformat(model_meta["last_sync"]),
            created=datetime.fromisoformat(model_meta["created"]),
        )

    def initialize_cache(self, model_id: str, dimensions: int) -> None:
        """Initialize cache database for a model."""
        db_path = self._get_cache_db_path(model_id)
        db = duckdb.connect(str(db_path))

        db.execute(
            dedent("""\
            CREATE TABLE IF NOT EXISTS embeddings (
                repo_id VARCHAR PRIMARY KEY,
                embedding JSON NOT NULL,
                content_hash VARCHAR NOT NULL,
                embedded_at TIMESTAMP NOT NULL,
                model_id VARCHAR NOT NULL,
                model_dimensions INTEGER NOT NULL
            )""")
        )

        db.execute(
            dedent("""\
            CREATE TABLE IF NOT EXISTS cache_metadata (
                model_id VARCHAR PRIMARY KEY,
                dimensions INTEGER NOT NULL,
                total_embeddings INTEGER NOT NULL,
                last_sync TIMESTAMP NOT NULL,
                created_at TIMESTAMP NOT NULL
            )""")
        )

        existing = db.execute(
            "SELECT COUNT(*) FROM cache_metadata WHERE model_id = ?", [model_id]
        ).fetchone()[0]

        if existing == 0:
            now = datetime.now()
            db.execute(
                dedent("""\
                INSERT INTO cache_metadata
                (model_id, dimensions, total_embeddings, last_sync, created_at)
                VALUES (?, ?, 0, ?, ?)"""),
                [model_id, dimensions, now, now],
            )

        db.close()

        metadata = self._load_metadata()
        if model_id not in metadata["models"]:
            metadata["models"][model_id] = {
                "db_path": db_path.name,
                "dimensions": dimensions,
                "total_repos": 0,
                "last_sync": datetime.now().isoformat(),
                "created": datetime.now().isoformat(),
            }
            self._save_metadata(metadata)

    def get_repos_needing_embeddings(
        self, main_db: duckdb.DuckDBPyConnection, model_id: str
    ) -> list[dict]:
        """Find repos that need embeddings (new or changed content).

        Returns list of repo dicts with: id, full_name, description,
        purpose, topics_array, content_hash.
        """
        cache_db_path = self._get_cache_db_path(model_id)

        if not cache_db_path.exists():
            repos = main_db.execute(
                dedent("""\
                SELECT id, full_name, description, purpose,
                       topics_array, content_hash
                FROM repositories
                ORDER BY id""")
            ).fetchall()

            return [_row_to_repo_dict(r) for r in repos]

        main_db.execute(f"ATTACH '{cache_db_path}' AS cache")

        repos = main_db.execute(
            dedent("""\
            SELECT r.id, r.full_name, r.description, r.purpose,
                   r.topics_array, r.content_hash
            FROM repositories r
            LEFT JOIN cache.embeddings e
                ON r.id = e.repo_id
                AND r.content_hash = e.content_hash
            WHERE e.repo_id IS NULL
            ORDER BY r.id""")
        ).fetchall()

        main_db.execute("DETACH cache")

        return [_row_to_repo_dict(r) for r in repos]

    def store_embeddings(
        self, model_id: str, dimensions: int, embeddings_data: list[dict]
    ) -> None:
        """Store embeddings in cache.

        embeddings_data: List of dicts with keys:
            - repo_id: str
            - embedding: list[float]
            - content_hash: str
        """
        db_path = self._get_cache_db_path(model_id)
        db = duckdb.connect(str(db_path))

        now = datetime.now()

        repo_ids = [data["repo_id"] for data in embeddings_data]
        db.execute(
            "DELETE FROM embeddings WHERE repo_id IN (SELECT UNNEST(?::VARCHAR[]))",
            [repo_ids],
        )

        rows = [
            (
                data["repo_id"],
                json.dumps(data["embedding"]),
                data["content_hash"],
                now,
                model_id,
                dimensions,
            )
            for data in embeddings_data
        ]
        db.executemany(
            dedent("""\
            INSERT INTO embeddings
            (repo_id, embedding, content_hash, embedded_at,
             model_id, model_dimensions)
            VALUES (?, ?::JSON, ?, ?, ?, ?)"""),
            rows,
        )

        total = db.execute("SELECT COUNT(*) FROM embeddings").fetchone()[0]

        db.execute(
            dedent("""\
            UPDATE cache_metadata
            SET total_embeddings = ?, last_sync = ?
            WHERE model_id = ?"""),
            [total, now, model_id],
        )

        db.close()

        metadata = self._load_metadata()
        if model_id in metadata["models"]:
            metadata["models"][model_id]["total_repos"] = total
            metadata["models"][model_id]["last_sync"] = now.isoformat()
            self._save_metadata(metadata)

    def create_eval_view(
        self, main_db: duckdb.DuckDBPyConnection, model_id: str, view_name: str
    ) -> None:
        """Create view in main DB pointing to cached embeddings.

        View schema: (repo_id VARCHAR, embedding JSON)
        """
        _validate_identifier(view_name)
        cache_name = f"{view_name}_cache"
        _validate_identifier(cache_name)

        cache_db_path = self._get_cache_db_path(model_id)

        if not cache_db_path.exists():
            raise ValueError(f"No cache exists for model: {model_id}")

        main_db.execute(f"ATTACH '{cache_db_path}' AS {cache_name}")
        main_db.execute(f"DROP VIEW IF EXISTS {view_name}")
        main_db.execute(
            dedent(f"""\
            CREATE VIEW {view_name} AS
            SELECT repo_id, embedding
            FROM {cache_name}.embeddings""")
        )

    def get_cache_stats(self) -> dict:
        """Get statistics for all cached models."""
        metadata = self._load_metadata()
        stats = {}

        for model_id, model_meta in metadata["models"].items():
            db_path = self.cache_dir / model_meta["db_path"]
            if db_path.exists():
                db = duckdb.connect(str(db_path))
                count = db.execute("SELECT COUNT(*) FROM embeddings").fetchone()[0]
                db.close()

                stats[model_id] = {
                    "embeddings_count": count,
                    "dimensions": model_meta["dimensions"],
                    "last_sync": model_meta["last_sync"],
                    "db_size_mb": db_path.stat().st_size / (1024 * 1024),
                }

        return stats


def sync_model_embeddings(
    main_db_path: str,
    model_id: str,
    dimensions: int,
    embedding_generator: Callable[[list[str]], list[list[float]]],
    batch_size: int = 128,
) -> int:
    """Incrementally sync embeddings for a model.

    Args:
        main_db_path: Path to main stars.db
        model_id: Model identifier (e.g., "intfloat/e5-small-v2")
        dimensions: Embedding dimensions
        embedding_generator: Callable that takes list of texts and returns embeddings
        batch_size: Batch size for embedding generation

    Returns:
        Number of repos embedded (0 if all cached)
    """
    cache = EmbeddingCache()
    cache.initialize_cache(model_id, dimensions)

    main_db = duckdb.connect(main_db_path)

    repos_to_embed = cache.get_repos_needing_embeddings(main_db, model_id)

    if not repos_to_embed:
        print(f"  All embeddings cached for {model_id}")
        return 0

    print(f"  Need to embed {len(repos_to_embed)} repos")

    from evaluate_embeddings import _build_embedding_input

    embeddings_data = []

    for i in range(0, len(repos_to_embed), batch_size):
        batch = repos_to_embed[i : i + batch_size]
        texts = [_build_embedding_input(repo) for repo in batch]

        embeddings = embedding_generator(texts)

        for repo, emb in zip(batch, embeddings, strict=True):
            embeddings_data.append(
                {
                    "repo_id": repo["id"],
                    "embedding": emb,
                    "content_hash": repo["content_hash"],
                }
            )

        batch_count = (len(repos_to_embed) + batch_size - 1) // batch_size
        print(
            f"    Embedded batch {i // batch_size + 1}/{batch_count}",
            end="\r",
        )

    print()

    cache.store_embeddings(model_id, dimensions, embeddings_data)

    main_db.close()

    return len(repos_to_embed)
