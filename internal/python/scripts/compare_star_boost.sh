#!/usr/bin/env bash
# Compare evaluation results with different star boost coefficients

set -e

DB_PATH="${1:-$HOME/.local/share/gh-star-search/stars.db}"
CONFIG="${2:-eval_config_enhanced.toml}"

echo "Comparing star boost coefficients..."
echo "Database: $DB_PATH"
echo "Config: $CONFIG"
echo ""

echo "==================================="
echo "BASELINE: Current boost (0.1)"
echo "==================================="

uv run python evaluate_embeddings.py \
    --config "$CONFIG" \
    --db "$DB_PATH" \
    --star-boost 0.1 \
    2>&1 | tee results_boost_0.1.log

# Save results
TIMESTAMP_1=$(ls -t eval_results/*.json | head -1 | xargs basename | sed 's/evaluation_//;s/.json//')
cp "eval_results/evaluation_${TIMESTAMP_1}.json" "eval_results/boost_0.1_${TIMESTAMP_1}.json"
cp "eval_results/evaluation_${TIMESTAMP_1}.md" "eval_results/boost_0.1_${TIMESTAMP_1}.md"

echo ""
echo "==================================="
echo "TEST: Reduced boost (0.05)"
echo "==================================="

uv run python evaluate_embeddings.py \
    --config "$CONFIG" \
    --db "$DB_PATH" \
    --star-boost 0.05 \
    2>&1 | tee results_boost_0.05.log

# Save results
TIMESTAMP_2=$(ls -t eval_results/*.json | head -1 | xargs basename | sed 's/evaluation_//;s/.json//')
cp "eval_results/evaluation_${TIMESTAMP_2}.json" "eval_results/boost_0.05_${TIMESTAMP_2}.json"
cp "eval_results/evaluation_${TIMESTAMP_2}.md" "eval_results/boost_0.05_${TIMESTAMP_2}.md"

echo ""
echo "==================================="
echo "COMPARISON SUMMARY"
echo "==================================="
echo ""
echo "Baseline (0.1):  eval_results/boost_0.1_${TIMESTAMP_1}.md"
echo "Reduced (0.05):  eval_results/boost_0.05_${TIMESTAMP_2}.md"
echo ""

# Extract key metrics
echo "Baseline (0.1) - e5-small-v2:"
grep -A 1 "| MRR" "eval_results/boost_0.1_${TIMESTAMP_1}.md" | head -2
echo ""
echo "Reduced (0.05) - e5-small-v2:"
grep -A 1 "| MRR" "eval_results/boost_0.05_${TIMESTAMP_2}.md" | head -2
echo ""

echo "Done! Review the .md files for full comparison."
