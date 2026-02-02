# Evaluation Results

This directory stores outputs from the embedding model evaluation tool.

## File Types

### JSON Results (gitignored)
- Pattern: `evaluation_YYYYMMDD_HHMMSS.json`
- Contains: Complete evaluation data including per-query metrics, top results, model configs
- Purpose: Machine-readable results for further analysis
- Version Control: **Not tracked** (`.gitignore` excludes `*.json`)

### Markdown Reports (tracked)
- Pattern: `evaluation_YYYYMMDD_HHMMSS.md`
- Contains: Summary metrics, statistical test results, recommendations, sample queries
- Purpose: Human-readable reports for decision-making and reference
- Version Control: **Tracked in VCS** for historical comparison

## Usage

After running `evaluate_embeddings.py`, this directory will contain:

```
eval_results/
├── evaluation_20260201_143022.json  # Detailed results (gitignored)
├── evaluation_20260201_143022.md    # Summary report (tracked)
└── README.md                         # This file
```

## Viewing Results

### Terminal Output
The tool displays results in the terminal using Rich formatting:
- Comparison table with all metrics
- Delta values showing improvement/regression
- Statistical test result
- Recommendation

### JSON Analysis
Use `jq` or Python to analyze detailed results:
```bash
jq '.models[0].aggregate_metrics' evaluation_*.json
jq '.models[0].per_query_results[] | select(.category == "composite")' evaluation_*.json
```

### Markdown Reports
View reports in any markdown viewer or editor. These are tracked in VCS
to maintain a history of model comparisons over time.

## Interpreting Metrics

- **MRR > 0.8**: Excellent - First relevant result typically in top 2
- **MRR 0.5-0.8**: Good - First relevant result typically in top 3-5
- **MRR < 0.5**: Needs improvement - Relevant results not ranking high enough

- **Precision@5 > 0.6**: High precision - Most top results are relevant
- **Precision@5 0.3-0.6**: Moderate precision - Mix of relevant/irrelevant
- **Precision@5 < 0.3**: Low precision - Too many irrelevant results

- **NDCG@10 > 0.7**: Strong ranking quality with graded relevance
- **NDCG@10 0.4-0.7**: Acceptable ranking but room for improvement
- **NDCG@10 < 0.4**: Poor ranking - highly relevant items not at top

## Statistical Significance

The tool uses paired t-test on per-query MRR values:
- **p < 0.05**: Statistically significant difference
- **p >= 0.05**: No significant difference (could be noise/chance)

Combined with effect size:
- **|ΔMRR| > 0.05 and p < 0.05**: Meaningful and significant change
- **|ΔMRR| < 0.05 or p >= 0.05**: No actionable difference

## Cleanup

To remove old results while keeping recent evaluations:
```bash
# Keep last 5 evaluations
ls -t evaluation_*.json | tail -n +6 | xargs rm -f
ls -t evaluation_*.md | tail -n +6 | xargs rm -f
```
