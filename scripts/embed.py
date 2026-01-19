#!/usr/bin/env python3
"""
Text Embedding Generation Script

Generates vector embeddings for text using sentence-transformers.
Designed for RAM efficiency with a lightweight model.

Model: all-MiniLM-L6-v2
- Dimensions: 384
- Size: ~80MB download, ~200MB RAM usage
- Quality: Good balance of speed and accuracy

Usage:
    python embed.py < input.txt
    python embed.py --json < input.txt
"""

import argparse
import sys
import json
from typing import List, Optional


def generate_embedding(text: str) -> Optional[List[float]]:
    """
    Generate an embedding vector for the given text.

    Args:
        text: Input text to embed

    Returns:
        List of floats representing the embedding vector,
        or None if generation fails
    """
    if not text or len(text.strip()) == 0:
        return None

    try:
        from sentence_transformers import SentenceTransformer

        # Load the model (cached after first use)
        # all-MiniLM-L6-v2: 384 dimensions, ~80MB download
        model = SentenceTransformer('sentence-transformers/all-MiniLM-L6-v2')

        # Generate embedding
        # Returns a numpy array, convert to list for JSON serialization
        embedding = model.encode(text, convert_to_numpy=True)

        # Convert to Python list of floats
        return embedding.tolist()

    except ImportError:
        print("Error: sentence-transformers library not found", file=sys.stderr)
        print("Install with: pip install sentence-transformers", file=sys.stderr)
        return None
    except Exception as e:
        print(f"Error: Failed to generate embedding: {e}", file=sys.stderr)
        return None


def main():
    parser = argparse.ArgumentParser(
        description="Generate vector embeddings for text using sentence-transformers"
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Output result as JSON with metadata"
    )
    parser.add_argument(
        "--model",
        default="sentence-transformers/all-MiniLM-L6-v2",
        help="Model to use (default: all-MiniLM-L6-v2)"
    )

    args = parser.parse_args()

    # Read input from stdin
    try:
        input_text = sys.stdin.read()
    except Exception as e:
        result = {
            "success": False,
            "error": f"Failed to read input: {str(e)}",
            "embedding": None,
            "dimensions": 0
        }
        print(json.dumps(result))
        sys.exit(1)

    # Generate embedding
    embedding = generate_embedding(input_text)

    if embedding is None:
        result = {
            "success": False,
            "error": "Failed to generate embedding",
            "embedding": None,
            "dimensions": 0
        }
        if args.json:
            print(json.dumps(result))
        else:
            sys.exit(1)
        sys.exit(1)

    # Output result
    if args.json:
        result = {
            "success": True,
            "embedding": embedding,
            "dimensions": len(embedding),
            "model": args.model,
            "input_length": len(input_text)
        }
        print(json.dumps(result))
    else:
        # Output as comma-separated floats
        print(",".join(str(x) for x in embedding))


if __name__ == "__main__":
    main()
