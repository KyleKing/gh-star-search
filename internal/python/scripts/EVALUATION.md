# Embedding Model Evaluation

Tool for comparing embedding models on real GitHub starred repository data.

## Quick Start

1. Ensure database is populated:
   ```bash
   gh star-search sync
   ```

2. Customize models in `eval_config.toml`:
   ```toml
   [models.current]
   name = "e5-small-v2"
   model_id = "intfloat/e5-small-v2"
   dimensions = 384

   [models.candidate]
   name = "bge-small"
   model_id = "BAAI/bge-small-en-v1.5"
   dimensions = 384
   ```

3. Run evaluation:
   ```bash
   cd internal/python/scripts
   uv run python evaluate_embeddings.py
   ```

## Configuration

### Test Queries (`eval_config.toml`)

30 curated queries covering:
- **Technology-focused**: "rust", "typescript", "vue"
- **Use-case focused**: "testing framework", "authentication", "data visualization"
- **Composite**: "rust cli tool", "python web framework", "react component library"
- **Edge cases**: Very short queries ("cli", "orm")

Each query includes:
- Expected relevant repositories
- Relevance grades (0-3 scale for NDCG calculation):
  - 3: Highly relevant (exact match)
  - 2: Relevant (good match)
  - 1: Marginally relevant (tangential)
  - 0: Irrelevant (default)

### Model Configuration

Any sentence-transformers compatible model can be evaluated. Key parameters:
- `name`: Display name
- `model_id`: HuggingFace model identifier
- `dimensions`: Embedding dimensions (must match model output)

## Metrics

### Mean Reciprocal Rank (MRR)
Position of first relevant result. Higher is better (max 1.0).
```
MRR = 1 / rank_of_first_relevant_result
```

### Precision@k
Fraction of relevant results in top k. Higher is better (max 1.0).
```
Precision@k = relevant_in_top_k / k
```

### Recall@k
Fraction of all relevant results found in top k. Higher is better (max 1.0).
```
Recall@k = relevant_in_top_k / total_relevant
```

### NDCG@k (Normalized Discounted Cumulative Gain)
Graded relevance metric accounting for position. Higher is better (max 1.0).
```
DCG@k = Σ(relevance_i / log2(i+1))
NDCG@k = DCG@k / IDCG@k
```

## Outputs

### Terminal Display
Rich formatted table with:
- Aggregate metrics for both models
- Delta between models
- Statistical significance test result
- Recommendation

### JSON Results (`eval_results/evaluation_YYYYMMDD.json`)
Complete evaluation data including:
- Per-query metrics for both models
- Top 5 results for each query
- Model configurations
- Statistical test details

**Note**: JSON files are gitignored to avoid repository bloat.

### Markdown Report (`eval_results/evaluation_YYYYMMDD.md`)
Summary report with:
- Aggregate metrics comparison
- Statistical test results
- Recommendation
- Sample query results

**Note**: Markdown reports are tracked in VCS for reference.

## Implementation Details

### Ranking Algorithm
Matches production query engine (`internal/query/engine.go`):
1. Base score: Cosine similarity between query and repo embeddings
2. Star boost: `1.0 + (0.1 * log10(stars + 1) / 6.0)`
3. Recency factor: `1.0 - 0.2 * min(1.0, days_since_update / 365)`
4. Final score: `base_score * star_boost * recency_factor`

### Embedding Input Format
Matches production sync logic (`cmd/sync_embed.go`):
```
{full_name}. {purpose}. {description}. {topics joined}
```

### Performance
- Processes ~7,000 repos in ~30-40 minutes
- Uses batched embedding generation (batch size: 32)
- Temporary tables for model comparisons
- Parallel processing where possible

## Statistical Testing

Paired t-test on per-query MRR values:
- Null hypothesis: No difference between models
- Alternative: Model B differs from Model A
- Significance threshold: p < 0.05
- Effect size threshold: |ΔMRR| > 0.05

Recommendation logic:
- **Significant improvement**: p < 0.05 and ΔMRR > 0.05
- **Significant regression**: p < 0.05 and ΔMRR < -0.05
- **No significant difference**: Otherwise

## Adding New Queries

Edit `eval_config.toml`:

```toml
[[queries]]
id = "q031"
query = "your query here"
category = "composite"  # or: technology, use_case, edge_case
expected_repos = ["owner/repo1", "owner/repo2"]
relevance_grades = { "owner/repo1" = 3, "owner/repo2" = 3, "owner/repo3" = 2 }
```

Best practices:
- Use diverse query types (short/long, specific/general)
- Include realistic user search patterns
- Set relevance grades based on actual match quality
- Test queries manually first to validate expectations

## Troubleshooting

### Database not found
Run `gh star-search sync` to create and populate database.

### Model download fails
Check internet connection and HuggingFace model availability.
Models are cached in `~/.cache/huggingface/`.

### Out of memory
Reduce batch size in `evaluate_embeddings.py` (line ~300):
```python
batch_size = 16  # Default: 32
```

### Slow evaluation
- Use smaller models (e.g., all-MiniLM-L6-v2 at 384d)
- Reduce number of queries in config
- Check system resources (CPU/memory)

## Future Enhancements

Potential improvements (out of scope for initial implementation):
- Multi-model comparison (>2 models)
- Automatic query generation from repo metadata
- HTML reports with interactive charts
- Trend tracking across evaluation runs
- CI integration for regression testing
- Category-specific metric breakdowns
