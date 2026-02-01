#!/usr/bin/env python3
"""
Lightweight embedding generation script for gh-star-search.

This script generates embeddings for text using sentence-transformers with
small models that work within 2-3GB of RAM.

Install dependencies:
    pip install sentence-transformers

The script uses all-MiniLM-L6-v2 by default (80MB model, 384-dim embeddings).
"""

import sys
import json
import argparse
from typing import List, Dict, Optional


def generate_embeddings(
    texts: List[str],
    model_name: str = "sentence-transformers/all-MiniLM-L6-v2"
) -> List[List[float]]:
    """
    Generate embeddings for a list of texts.

    Args:
        texts: List of texts to embed
        model_name: Name of the sentence-transformers model to use

    Returns:
        List of embedding vectors (each is a list of floats)
    """
    try:
        from sentence_transformers import SentenceTransformer
    except ImportError:
        raise ImportError(
            "sentence-transformers not installed. "
            "Install with: pip install sentence-transformers"
        )

    # Load model (will download on first use, ~80MB for default model)
    model = SentenceTransformer(model_name)

    # Generate embeddings
    embeddings = model.encode(texts, show_progress_bar=False)

    # Convert to list of lists
    return embeddings.tolist()


def main():
    """Main entry point for CLI usage."""
    parser = argparse.ArgumentParser(
        description="Generate embeddings for gh-star-search"
    )
    parser.add_argument(
        "texts",
        nargs="*",
        help="Texts to embed (or read from stdin as JSON array)"
    )
    parser.add_argument(
        "--model",
        default="sentence-transformers/all-MiniLM-L6-v2",
        help="Model to use (default: all-MiniLM-L6-v2)"
    )
    parser.add_argument(
        "--stdin",
        action="store_true",
        help="Read texts from stdin as JSON array"
    )

    args = parser.parse_args()

    # Get texts
    if args.stdin or not args.texts:
        # Read from stdin
        input_data = sys.stdin.read()
        try:
            texts = json.loads(input_data)
            if not isinstance(texts, list):
                print("Error: stdin must contain a JSON array", file=sys.stderr)
                sys.exit(1)
        except json.JSONDecodeError as e:
            print(f"Error: invalid JSON: {e}", file=sys.stderr)
            sys.exit(1)
    else:
        texts = args.texts

    # Generate embeddings
    try:
        embeddings = generate_embeddings(texts, args.model)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

    # Output as JSON
    result = {
        "embeddings": embeddings,
        "model": args.model,
        "dimension": len(embeddings[0]) if embeddings else 0,
        "count": len(embeddings)
    }

    print(json.dumps(result))


if __name__ == "__main__":
    main()
