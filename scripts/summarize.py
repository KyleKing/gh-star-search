#!/usr/bin/env python3
"""
Lightweight extractive summarization script for gh-star-search.

This script uses simple extractive summarization techniques that work
within 2-3GB of RAM. It avoids heavy transformer models by default
and uses a heuristic-based approach.

For better results with slightly more RAM, install transformers:
    pip install transformers torch

The script will automatically use the best available method.
"""

import sys
import json
import argparse
from typing import List, Dict, Optional
import re


def simple_sentence_score(sentence: str, keywords: List[str]) -> float:
    """
    Score a sentence based on keyword presence and position.

    Args:
        sentence: The sentence to score
        keywords: Important keywords to look for

    Returns:
        Score between 0 and 1
    """
    score = 0.0
    sentence_lower = sentence.lower()

    # Keyword presence (40% weight)
    keyword_count = sum(1 for kw in keywords if kw.lower() in sentence_lower)
    if keywords:
        score += (keyword_count / len(keywords)) * 0.4

    # Sentence position (30% weight) - earlier sentences are more important
    # This is a placeholder - actual position should be passed in
    score += 0.3  # Assume mid-document for simplicity

    # Sentence length (30% weight) - prefer medium-length sentences
    word_count = len(sentence.split())
    if 10 <= word_count <= 25:
        score += 0.3
    elif 5 <= word_count < 10 or 25 < word_count <= 35:
        score += 0.2
    elif word_count < 5:
        score += 0.0
    else:
        score += 0.1

    return score


def extract_keywords(text: str, top_n: int = 10) -> List[str]:
    """
    Extract keywords from text using simple frequency analysis.

    Args:
        text: The text to analyze
        top_n: Number of keywords to extract

    Returns:
        List of top keywords
    """
    # Simple stopwords
    stopwords = {
        'the', 'a', 'an', 'and', 'or', 'but', 'in', 'on', 'at', 'to', 'for',
        'of', 'with', 'is', 'are', 'was', 'were', 'be', 'been', 'being',
        'have', 'has', 'had', 'do', 'does', 'did', 'will', 'would', 'could',
        'should', 'may', 'might', 'can', 'this', 'that', 'these', 'those',
        'it', 'its', 'as', 'by', 'from', 'into', 'through', 'during', 'before',
        'after', 'above', 'below', 'up', 'down', 'out', 'off', 'over', 'under',
        'again', 'further', 'then', 'once', 'here', 'there', 'when', 'where',
        'why', 'how', 'all', 'both', 'each', 'few', 'more', 'most', 'other',
        'some', 'such', 'than', 'too', 'very', 'just', 'only', 'own', 'same'
    }

    # Tokenize and filter
    words = re.findall(r'\b[a-z]{3,}\b', text.lower())
    words = [w for w in words if w not in stopwords]

    # Count frequencies
    word_freq = {}
    for word in words:
        word_freq[word] = word_freq.get(word, 0) + 1

    # Get top N
    sorted_words = sorted(word_freq.items(), key=lambda x: x[1], reverse=True)
    return [word for word, _ in sorted_words[:top_n]]


def split_sentences(text: str) -> List[str]:
    """
    Split text into sentences using simple rules.

    Args:
        text: The text to split

    Returns:
        List of sentences
    """
    # Simple sentence splitting
    sentences = re.split(r'[.!?]+', text)
    sentences = [s.strip() for s in sentences if s.strip()]
    return sentences


def heuristic_summarize(text: str, max_sentences: int = 3) -> str:
    """
    Perform heuristic-based extractive summarization.

    This method uses keyword extraction and sentence scoring to select
    the most important sentences. Very lightweight - uses minimal RAM.

    Args:
        text: The text to summarize
        max_sentences: Maximum number of sentences in summary

    Returns:
        Summary text
    """
    if not text or len(text.strip()) < 50:
        return text.strip()

    # Extract keywords
    keywords = extract_keywords(text)

    # Split into sentences
    sentences = split_sentences(text)

    if len(sentences) <= max_sentences:
        return text.strip()

    # Score sentences
    scored_sentences = []
    for i, sentence in enumerate(sentences):
        # Add position bonus (earlier = better)
        position_bonus = 1.0 - (i / len(sentences)) * 0.3
        score = simple_sentence_score(sentence, keywords) * position_bonus
        scored_sentences.append((score, sentence))

    # Sort by score and take top N
    scored_sentences.sort(reverse=True, key=lambda x: x[0])
    top_sentences = [sent for _, sent in scored_sentences[:max_sentences]]

    # Reorder by original position for coherence
    result_sentences = []
    for sent in sentences:
        if sent in top_sentences:
            result_sentences.append(sent)
            if len(result_sentences) == max_sentences:
                break

    return '. '.join(result_sentences) + '.'


def try_transformers_summarize(text: str, max_length: int = 150) -> Optional[str]:
    """
    Try to use transformers library for better summarization.

    Uses a small model (sshleifer/distilbart-cnn-12-6) that fits in ~2GB RAM.
    Falls back to None if transformers is not available.

    Args:
        text: The text to summarize
        max_length: Maximum length of summary in tokens

    Returns:
        Summary text or None if transformers unavailable
    """
    try:
        from transformers import pipeline
        import torch

        # Use a small model that fits in limited RAM
        # distilbart is much smaller than full BART (~1.5GB vs 6GB)
        model_name = "sshleifer/distilbart-cnn-12-6"

        # Set device to CPU to avoid GPU memory issues
        device = -1  # CPU

        # Create summarization pipeline with limited RAM usage
        summarizer = pipeline(
            "summarization",
            model=model_name,
            device=device,
            torch_dtype=torch.float32  # Use float32 instead of float16 for CPU
        )

        # Limit input length to avoid memory issues
        max_input_length = 1024
        if len(text.split()) > max_input_length:
            # Take first part only
            words = text.split()[:max_input_length]
            text = ' '.join(words)

        # Generate summary
        result = summarizer(
            text,
            max_length=max_length,
            min_length=30,
            do_sample=False,  # Deterministic
            truncation=True
        )

        return result[0]['summary_text']

    except ImportError:
        # Transformers not installed
        return None
    except Exception as e:
        # Any other error (OOM, model download failure, etc.)
        print(f"Warning: Transformers summarization failed: {e}", file=sys.stderr)
        return None


def summarize(
    text: str,
    method: str = "auto",
    max_sentences: int = 3,
    max_length: int = 150
) -> Dict[str, str]:
    """
    Summarize text using the specified method.

    Args:
        text: The text to summarize
        method: "auto", "heuristic", or "transformers"
        max_sentences: For heuristic method
        max_length: For transformers method

    Returns:
        Dict with 'summary' and 'method' keys
    """
    actual_method = "heuristic"
    summary = ""

    if method == "auto":
        # Try transformers first, fall back to heuristic
        summary = try_transformers_summarize(text, max_length)
        if summary:
            actual_method = "transformers"
        else:
            summary = heuristic_summarize(text, max_sentences)
            actual_method = "heuristic"
    elif method == "transformers":
        summary = try_transformers_summarize(text, max_length)
        if not summary:
            raise ValueError("Transformers method requested but unavailable")
        actual_method = "transformers"
    else:  # heuristic
        summary = heuristic_summarize(text, max_sentences)
        actual_method = "heuristic"

    return {
        "summary": summary,
        "method": actual_method
    }


def main():
    """Main entry point for CLI usage."""
    parser = argparse.ArgumentParser(
        description="Lightweight extractive summarization for gh-star-search"
    )
    parser.add_argument(
        "text",
        nargs="?",
        help="Text to summarize (or read from stdin)"
    )
    parser.add_argument(
        "--method",
        choices=["auto", "heuristic", "transformers"],
        default="auto",
        help="Summarization method (default: auto)"
    )
    parser.add_argument(
        "--max-sentences",
        type=int,
        default=3,
        help="Maximum sentences for heuristic method (default: 3)"
    )
    parser.add_argument(
        "--max-length",
        type=int,
        default=150,
        help="Maximum length for transformers method (default: 150)"
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Output as JSON"
    )

    args = parser.parse_args()

    # Get text from args or stdin
    if args.text:
        text = args.text
    else:
        text = sys.stdin.read()

    # Summarize
    result = summarize(
        text,
        method=args.method,
        max_sentences=args.max_sentences,
        max_length=args.max_length
    )

    # Output
    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(result["summary"])


if __name__ == "__main__":
    main()
