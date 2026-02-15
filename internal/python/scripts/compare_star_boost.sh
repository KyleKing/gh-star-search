#!/usr/bin/env bash
# Compare evaluation results with different star boost coefficients

set -e

DB_PATH="${1:-$HOME/.config/gh-star-search/database.db}"
CONFIG="${2:-eval_config_enhanced.toml}"

echo "Comparing star boost coefficients..."
echo "Database: $DB_PATH"
echo "Config: $CONFIG"
echo ""

# Backup original file
cp evaluate_embeddings.py evaluate_embeddings.py.backup

echo "==================================="
echo "BASELINE: Current boost (0.1)"
echo "==================================="
# Restore original if needed
cp evaluate_embeddings.py.backup evaluate_embeddings.py

# Run with current boost
uv run python evaluate_embeddings.py \
    --config "$CONFIG" \
    --db "$DB_PATH" \
    2>&1 | tee results_boost_0.1.log

# Save results
TIMESTAMP_1=$(ls -t eval_results/*.json | head -1 | xargs basename | sed 's/evaluation_//;s/.json//')
cp "eval_results/evaluation_${TIMESTAMP_1}.json" "eval_results/boost_0.1_${TIMESTAMP_1}.json"
cp "eval_results/evaluation_${TIMESTAMP_1}.md" "eval_results/boost_0.1_${TIMESTAMP_1}.md"

echo ""
echo "==================================="
echo "TEST: Reduced boost (0.05)"
echo "==================================="

# Modify star boost coefficient
sed -i.bak 's/star_boost = 1.0 + (0.1 \* math.log10/star_boost = 1.0 + (0.05 * math.log10/' evaluate_embeddings.py

# Run with reduced boost
uv run python evaluate_embeddings.py \
    --config "$CONFIG" \
    --db "$DB_PATH" \
    2>&1 | tee results_boost_0.05.log

# Save results
TIMESTAMP_2=$(ls -t eval_results/*.json | head -1 | xargs basename | sed 's/evaluation_//;s/.json//')
cp "eval_results/evaluation_${TIMESTAMP_2}.json" "eval_results/boost_0.05_${TIMESTAMP_2}.json"
cp "eval_results/evaluation_${TIMESTAMP_2}.md" "eval_results/boost_0.05_${TIMESTAMP_2}.md"

# Restore original
mv evaluate_embeddings.py.backup evaluate_embeddings.py

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
