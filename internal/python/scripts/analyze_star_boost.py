#!/usr/bin/env python3
"""Analyze impact of star boost coefficient on ranking."""

import argparse
import math

import duckdb
from evaluate_embeddings import DEFAULT_DB_PATH


def compute_star_boost(stars: int, coefficient: float = 0.1) -> float:
    """Compute star boost with given coefficient."""
    return 1.0 + (coefficient * math.log10(stars + 1) / 6.0)


def analyze_boost_impact(db_path: str) -> None:
    """Analyze how star boost affects ranking across database."""
    db = duckdb.connect(db_path)

    # Get star distribution
    repos = db.execute("""
        SELECT full_name, stargazers_count
        FROM repositories
        ORDER BY stargazers_count DESC
    """).fetchall()

    print("Star Boost Analysis")
    print("=" * 80)
    print("\nBoost by Star Count:")
    print(
        f"{'Stars':>10} | {'Current (0.1)':>15} | "
        f"{'Reduced (0.05)':>15} | {'Difference':>12}"
    )
    print("-" * 80)

    test_values = [10, 50, 100, 500, 1000, 5000, 10000, 50000, 100000]
    for stars in test_values:
        current = compute_star_boost(stars, 0.1)
        reduced = compute_star_boost(stars, 0.05)
        diff = current - reduced
        print(f"{stars:10,} | {current:15.3f} | {reduced:15.3f} | {diff:12.3f}")

    print("\n" + "=" * 80)
    print(f"\nYour Repository Distribution ({len(repos)} repos):")
    print(f"{'Repo':40} | {'Stars':>8} | {'Current':>10} | {'Reduced':>10}")
    print("-" * 80)

    for full_name, stars in repos[:15]:  # Top 15
        current = compute_star_boost(stars, 0.1)
        reduced = compute_star_boost(stars, 0.05)
        short_name = full_name if len(full_name) <= 40 else full_name[:37] + "..."
        print(f"{short_name:40} | {stars:8,} | {current:10.3f} | {reduced:10.3f}")

    # Calculate percentile impacts
    print("\n" + "=" * 80)
    print("\nPercentile Impact:")
    star_counts = [r[1] for r in repos]
    percentiles = [10, 25, 50, 75, 90, 95, 99]

    print(f"{'Percentile':>10} | {'Stars':>10} | {'Current':>10} | {'Reduced':>10}")
    print("-" * 80)

    for p in percentiles:
        idx = int(len(star_counts) * p / 100)
        if idx >= len(star_counts):
            idx = len(star_counts) - 1
        stars = star_counts[idx]
        current = compute_star_boost(stars, 0.1)
        reduced = compute_star_boost(stars, 0.05)
        print(f"{p:10}th | {stars:10,} | {current:10.3f} | {reduced:10.3f}")

    db.close()


def compare_ranking_impact(
    db_path: str, query_embedding: list[float], query_name: str
) -> None:
    """Compare top-10 results with different star boost coefficients."""
    db = duckdb.connect(db_path)

    print(f"\n{'=' * 80}")
    print(f"Ranking Impact for Query: {query_name}")
    print("=" * 80)

    # This would need actual embeddings - showing the concept
    print("\nNote: This requires embeddings to be loaded.")
    print("Run actual evaluation to see ranking differences.")

    db.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Analyze star boost impact")
    parser.add_argument(
        "--db",
        default=DEFAULT_DB_PATH,
        help="Path to database",
    )
    args = parser.parse_args()

    analyze_boost_impact(args.db)
