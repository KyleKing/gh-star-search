#!/usr/bin/env python3
"""Lightweight extractive summarization script for gh-star-search.

Uses simple extractive summarization techniques that work within 2-3GB of RAM.
Supports heuristic-based and transformers-based methods.
"""

import sys
import json
import argparse
import re
from typing import Optional


def simple_sentence_score(sentence: str, keywords: list[str]) -> float:
    """Score a sentence based on keyword presence and position.

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
    score += 0.3

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


def extract_keywords(text: str, top_n: int = 10) -> list[str]:
    """Extract keywords from text using simple frequency analysis.

    Args:
        text: The text to analyze
        top_n: Number of keywords to extract

    Returns:
        List of top keywords
    """
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

    words = re.findall(r'\b[a-z]{3,}\b', text.lower())
    words = [w for w in words if w not in stopwords]

    word_freq: dict[str, int] = {}
    for word in words:
        word_freq[word] = word_freq.get(word, 0) + 1

    sorted_words = sorted(word_freq.items(), key=lambda x: x[1], reverse=True)
    return [word for word, _ in sorted_words[:top_n]]


def split_sentences(text: str) -> list[str]:
    """Split text into sentences using simple rules.

    Args:
        text: The text to split

    Returns:
        List of sentences
    """
    sentences = re.split(r'[.!?]+', text)
    return [s.strip() for s in sentences if s.strip()]


def heuristic_summarize(text: str, max_sentences: int = 3) -> str:
    """Perform heuristic-based extractive summarization.

    Args:
        text: The text to summarize
        max_sentences: Maximum number of sentences in summary

    Returns:
        Summary text
    """
    if not text or len(text.strip()) < 50:
        return text.strip()

    keywords = extract_keywords(text)
    sentences = split_sentences(text)

    if len(sentences) <= max_sentences:
        return text.strip()

    scored_sentences = []
    for i, sentence in enumerate(sentences):
        position_bonus = 1.0 - (i / len(sentences)) * 0.3
        score = simple_sentence_score(sentence, keywords) * position_bonus
        scored_sentences.append((score, sentence))

    scored_sentences.sort(reverse=True, key=lambda x: x[0])
    top_sentences = [sent for _, sent in scored_sentences[:max_sentences]]

    result_sentences = []
    for sent in sentences:
        if sent in top_sentences:
            result_sentences.append(sent)
            if len(result_sentences) == max_sentences:
                break

    return '. '.join(result_sentences) + '.'


def _transformers_summarize(text: str, max_length: int = 150) -> Optional[str]:
    """Use transformers library for summarization.

    Args:
        text: The text to summarize
        max_length: Maximum length of summary in tokens

    Returns:
        Summary text or None on failure
    """
    try:
        from transformers import pipeline
        import torch

        model_name = "sshleifer/distilbart-cnn-12-6"
        device = -1

        summarizer = pipeline(
            "summarization",
            model=model_name,
            device=device,
            dtype=torch.float32,
        )

        max_input_length = 1024
        if len(text.split()) > max_input_length:
            words = text.split()[:max_input_length]
            text = ' '.join(words)

        result = summarizer(
            text,
            max_length=max_length,
            min_length=30,
            do_sample=False,
            truncation=True,
        )

        return result[0]['summary_text']
    except Exception as err:
        print(f"Warning: Transformers summarization failed: {err}", file=sys.stderr)
        return None


def summarize(
    text: str,
    method: str = "auto",
    max_sentences: int = 3,
    max_length: int = 150,
) -> dict[str, str]:
    """Summarize text using the specified method.

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

    match method:
        case "auto":
            if summary := _transformers_summarize(text, max_length):
                actual_method = "transformers"
            else:
                summary = heuristic_summarize(text, max_sentences)
                actual_method = "heuristic"
        case "transformers":
            if not (summary := _transformers_summarize(text, max_length)):
                raise ValueError("Transformers method requested but failed")
            actual_method = "transformers"
        case _:
            summary = heuristic_summarize(text, max_sentences)
            actual_method = "heuristic"

    return {
        "summary": summary,
        "method": actual_method,
    }


def main():
    parser = argparse.ArgumentParser(
        description="Lightweight extractive summarization for gh-star-search"
    )
    parser.add_argument(
        "text",
        nargs="?",
        help="Text to summarize (or read from stdin)",
    )
    parser.add_argument(
        "--method",
        choices=["auto", "heuristic", "transformers"],
        default="auto",
        help="Summarization method (default: auto)",
    )
    parser.add_argument(
        "--max-sentences",
        type=int,
        default=3,
        help="Maximum sentences for heuristic method (default: 3)",
    )
    parser.add_argument(
        "--max-length",
        type=int,
        default=150,
        help="Maximum length for transformers method (default: 150)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Output as JSON",
    )

    args = parser.parse_args()

    text = args.text if args.text else sys.stdin.read()

    result = summarize(
        text,
        method=args.method,
        max_sentences=args.max_sentences,
        max_length=args.max_length,
    )

    if args.json:
        print(json.dumps(result, indent=2))
    else:
        print(result["summary"])


if __name__ == "__main__":
    main()
