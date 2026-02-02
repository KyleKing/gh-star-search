#!/usr/bin/env python3
"""Embedding generation script for gh-star-search.

Generates embeddings for text using sentence-transformers with small models
that work within 2-3GB of RAM.

Default model: e5-small-v2 (118M params, 384-dim embeddings).
"""

import sys
import json
import argparse

from sentence_transformers import SentenceTransformer


def generate_embeddings(
    texts: list[str],
    model_name: str = "intfloat/e5-small-v2",
) -> list[list[float]]:
    """Generate embeddings for a list of texts.

    Args:
        texts: List of texts to embed
        model_name: Name of the sentence-transformers model to use

    Returns:
        List of embedding vectors (each is a list of floats)
    """
    model = SentenceTransformer(model_name)
    embeddings = model.encode(texts, show_progress_bar=False)
    return embeddings.tolist()


def main():
    parser = argparse.ArgumentParser(
        description="Generate embeddings for gh-star-search"
    )
    parser.add_argument(
        "texts",
        nargs="*",
        help="Texts to embed (or read from stdin as JSON array)",
    )
    parser.add_argument(
        "--model",
        default="intfloat/e5-small-v2",
        help="Model to use (default: e5-small-v2)",
    )
    parser.add_argument(
        "--stdin",
        action="store_true",
        help="Read texts from stdin as JSON array",
    )

    args = parser.parse_args()

    if args.stdin or not args.texts:
        input_data = sys.stdin.read()
        try:
            texts = json.loads(input_data)
            if not isinstance(texts, list):
                print("Error: stdin must contain a JSON array", file=sys.stderr)
                sys.exit(1)
        except json.JSONDecodeError as err:
            print(f"Error: invalid JSON: {err}", file=sys.stderr)
            sys.exit(1)
    else:
        texts = args.texts

    try:
        embeddings = generate_embeddings(texts, args.model)
    except Exception as err:
        print(f"Error: {err}", file=sys.stderr)
        sys.exit(1)

    result = {
        "embeddings": embeddings,
        "model": args.model,
        "dimension": len(embeddings[0]) if embeddings else 0,
        "count": len(embeddings),
    }

    print(json.dumps(result))


if __name__ == "__main__":
    main()
