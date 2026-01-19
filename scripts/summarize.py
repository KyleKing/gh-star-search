#!/usr/bin/env python3
"""
Repository Summarization Script

Generates concise summaries of repository content with RAM-efficient operation.
Supports two methods:
1. Heuristic (default): Simple keyword extraction (~10MB RAM)
2. Transformers: AI-based summarization using DistilBART (~1.5GB RAM)

Usage:
    python summarize.py --method heuristic < input.txt
    python summarize.py --method transformers < input.txt
"""

import argparse
import sys
import json
from typing import Optional, Dict, Any
import re
from collections import Counter


def heuristic_summarize(text: str, max_length: int = 200) -> str:
    """
    Generate a summary using simple heuristic methods.

    This method:
    1. Extracts key sentences based on word frequency
    2. Prioritizes sentences with technical keywords
    3. Returns the most relevant sentences up to max_length

    Args:
        text: Input text to summarize
        max_length: Maximum length of summary in characters

    Returns:
        Concise summary string
    """
    if not text or len(text.strip()) == 0:
        return ""

    # Split into sentences
    sentences = re.split(r'[.!?]+', text)
    sentences = [s.strip() for s in sentences if len(s.strip()) > 10]

    if not sentences:
        return text[:max_length]

    # Technical keywords that indicate important content
    tech_keywords = {
        'api', 'library', 'framework', 'tool', 'cli', 'server', 'client',
        'application', 'app', 'service', 'platform', 'system', 'engine',
        'implementation', 'interface', 'protocol', 'algorithm', 'data',
        'package', 'module', 'plugin', 'extension', 'utility', 'helper'
    }

    # Action verbs that indicate purpose
    action_verbs = {
        'provides', 'enables', 'allows', 'helps', 'creates', 'builds',
        'generates', 'manages', 'handles', 'processes', 'implements',
        'supports', 'offers', 'facilitates'
    }

    # Score each sentence
    scored_sentences = []
    for sentence in sentences:
        words = set(re.findall(r'\b\w+\b', sentence.lower()))

        # Calculate scores based on different factors
        tech_score = len(words & tech_keywords) * 3
        action_score = len(words & action_verbs) * 2
        length_score = min(len(sentence) / 50, 2)  # Prefer medium-length sentences
        position_score = 5 if sentence == sentences[0] else 0  # Boost first sentence

        total_score = tech_score + action_score + length_score + position_score
        scored_sentences.append((total_score, sentence))

    # Sort by score and take top sentences
    scored_sentences.sort(reverse=True, key=lambda x: x[0])

    # Build summary
    summary_parts = []
    current_length = 0

    for score, sentence in scored_sentences:
        sentence_text = sentence.strip() + '.'
        if current_length + len(sentence_text) <= max_length:
            summary_parts.append(sentence_text)
            current_length += len(sentence_text) + 1  # +1 for space
        if current_length >= max_length * 0.8:  # Stop at 80% to avoid over-filling
            break

    if not summary_parts:
        # Fall back to first sentence if nothing scored well
        return sentences[0][:max_length]

    return ' '.join(summary_parts)


def try_transformers_summarize(text: str, max_length: int = 200) -> Optional[str]:
    """
    Attempt to use transformers library for AI-based summarization.

    Uses DistilBART model which is relatively lightweight (~1.5GB RAM).
    Falls back to None if transformers is not available.

    Args:
        text: Input text to summarize
        max_length: Maximum length of summary in characters

    Returns:
        Summary string if successful, None if transformers unavailable
    """
    try:
        from transformers import pipeline
        import torch

        # Use a lightweight model that fits in ~1.5GB RAM
        summarizer = pipeline(
            "summarization",
            model="sshleifer/distilbart-cnn-6-6",
            device=-1  # Use CPU to avoid GPU memory issues
        )

        # Limit input to avoid memory issues (roughly 1000 tokens)
        max_input_length = 4000
        truncated_text = text[:max_input_length]

        # Generate summary
        # DistilBART works with word counts, convert char limit to approximate words
        max_words = max_length // 5  # Rough estimate: 5 chars per word
        min_words = max_words // 2

        result = summarizer(
            truncated_text,
            max_length=max_words,
            min_length=min_words,
            do_sample=False,
            truncation=True
        )

        if result and len(result) > 0:
            summary = result[0]['summary_text']
            # Trim to character limit if needed
            if len(summary) > max_length:
                summary = summary[:max_length-3] + '...'
            return summary

    except ImportError:
        # transformers not installed
        return None
    except Exception as e:
        # Any other error (memory, model loading, etc.)
        print(f"Warning: Transformers summarization failed: {e}", file=sys.stderr)
        return None

    return None


def summarize(text: str, method: str = "heuristic", max_length: int = 200) -> Dict[str, Any]:
    """
    Summarize text using the specified method.

    Args:
        text: Input text to summarize
        method: Either "heuristic" or "transformers"
        max_length: Maximum length of summary

    Returns:
        Dictionary with summary and metadata
    """
    if not text or len(text.strip()) == 0:
        return {
            "success": False,
            "error": "Empty input text",
            "summary": "",
            "method": method
        }

    summary = None
    actual_method = method

    if method == "transformers":
        summary = try_transformers_summarize(text, max_length)
        if summary is None:
            # Fall back to heuristic if transformers fails
            print("Falling back to heuristic method", file=sys.stderr)
            actual_method = "heuristic"

    if summary is None:
        summary = heuristic_summarize(text, max_length)
        actual_method = "heuristic"

    return {
        "success": True,
        "summary": summary,
        "method": actual_method,
        "input_length": len(text),
        "output_length": len(summary)
    }


def main():
    parser = argparse.ArgumentParser(
        description="Generate repository summaries with RAM-efficient methods"
    )
    parser.add_argument(
        "--method",
        choices=["heuristic", "transformers"],
        default="heuristic",
        help="Summarization method to use (default: heuristic)"
    )
    parser.add_argument(
        "--max-length",
        type=int,
        default=200,
        help="Maximum length of summary in characters (default: 200)"
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Output result as JSON"
    )

    args = parser.parse_args()

    # Read input from stdin
    try:
        input_text = sys.stdin.read()
    except Exception as e:
        result = {
            "success": False,
            "error": f"Failed to read input: {str(e)}",
            "summary": "",
            "method": args.method
        }
        print(json.dumps(result))
        sys.exit(1)

    # Generate summary
    result = summarize(input_text, args.method, args.max_length)

    # Output result
    if args.json:
        print(json.dumps(result, indent=2))
    else:
        if result["success"]:
            print(result["summary"])
        else:
            print(f"Error: {result.get('error', 'Unknown error')}", file=sys.stderr)
            sys.exit(1)


if __name__ == "__main__":
    main()
