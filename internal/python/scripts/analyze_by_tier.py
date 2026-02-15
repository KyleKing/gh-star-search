#!/usr/bin/env python3
"""Analyze evaluation results broken down by difficulty tier and intent."""

import json
import sys
from collections import defaultdict


def analyze_by_tier(results_file: str):
    """Analyze metrics by difficulty tier and intent."""
    with open(results_file) as f:
        data = json.load(f)

    # Group results by tier and intent
    by_tier = defaultdict(list)
    by_intent = defaultdict(list)

    for model_data in data["models"]:
        model_name = model_data["model_name"]
        print(f"\n{'=' * 80}")
        print(f"Model: {model_name}")
        print("=" * 80)

        for query_result in model_data["per_query_results"]:
            query_id = query_result["query_id"]
            category = query_result["category"]
            mrr = query_result["metrics"]["mrr"]

            # Extract difficulty and intent from category
            # Format: "easy_direct", "medium_composite", "hard_learning", etc.
            parts = category.split("_")
            difficulty = (
                parts[0] if parts[0] in ["easy", "medium", "hard"] else "unknown"
            )
            intent_cat = "_".join(parts[1:]) if len(parts) > 1 else category

            by_tier[difficulty].append(mrr)

            # Track intent if it exists
            if intent_cat:
                by_intent[intent_cat].append(mrr)

        # Print tier breakdown
        print("\nResults by Difficulty Tier:")
        print(f"{'Tier':<15} | {'Queries':>8} | {'Avg MRR':>10} | {'Success Rate':>15}")
        print("-" * 65)

        for tier in ["easy", "medium", "hard"]:
            if by_tier.get(tier):
                queries = len(by_tier[tier])
                avg_mrr = sum(by_tier[tier]) / queries
                success_rate = sum(1 for m in by_tier[tier] if m >= 0.5) / queries
                print(
                    f"{tier.capitalize():<15} | {queries:8} | {avg_mrr:10.3f} | {success_rate:14.1%}"
                )

        # Print intent breakdown
        print("\nResults by Intent:")
        print(
            f"{'Intent':<20} | {'Queries':>8} | {'Avg MRR':>10} | {'Success Rate':>15}"
        )
        print("-" * 70)

        for intent, mrrs in sorted(by_intent.items()):
            if mrrs:
                queries = len(mrrs)
                avg_mrr = sum(mrrs) / queries
                success_rate = sum(1 for m in mrrs if m >= 0.5) / queries
                print(
                    f"{intent:<20} | {queries:8} | {avg_mrr:10.3f} | {success_rate:14.1%}"
                )

        # Clear for next model
        by_tier.clear()
        by_intent.clear()


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python analyze_by_tier.py <results.json>")
        sys.exit(1)

    analyze_by_tier(sys.argv[1])
