# Python Scripts for gh-star-search

This directory contains Python scripts that provide optional enhanced functionality for gh-star-search.

## Overview

These scripts enable:
- **Summarization**: Extract key information from repository READMEs
- **Embeddings**: Generate vector embeddings for semantic search

Both scripts are designed to work within **2-3GB of RAM** and provide graceful fallbacks.

## Installation

### Minimal Installation (Heuristic Summarization Only)

No additional dependencies needed! The summarization script includes a built-in heuristic method that requires only Python 3.

### Enhanced Installation (Transformers + Embeddings)

For better summarization and vector search support:

```bash
# Install transformers for better summarization (~1.5GB model)
pip install transformers torch

# Install sentence-transformers for embeddings (~80MB model)
pip install sentence-transformers
```

## Scripts

### summarize.py

Extracts summaries from repository content.

**Methods:**
- `heuristic`: Keyword-based extractive summarization (no dependencies, ~10MB RAM)
- `transformers`: Uses DistilBART model (~1.5GB RAM, better quality)
- `auto`: Tries transformers, falls back to heuristic

**Usage:**
```bash
# CLI usage
python3 scripts/summarize.py "Your text here"

# With specific method
python3 scripts/summarize.py --method heuristic "Your text here"

# JSON output
python3 scripts/summarize.py --json "Your text here"

# From stdin
cat README.md | python3 scripts/summarize.py

# With parameters
python3 scripts/summarize.py --max-sentences 5 "Long text..."
```

**Memory Usage:**
- Heuristic: ~10MB
- Transformers: ~1.5-2GB (DistilBART model)

### embed.py

Generates vector embeddings for semantic search.

**Models:**
- Default: `sentence-transformers/all-MiniLM-L6-v2` (384 dimensions, ~80MB)
- Alternative: `sentence-transformers/all-mpnet-base-v2` (768 dimensions, ~420MB)

**Usage:**
```bash
# Single text
echo '["Your text here"]' | python3 scripts/embed.py --stdin

# Multiple texts
echo '["Text 1", "Text 2", "Text 3"]' | python3 scripts/embed.py --stdin

# Custom model
echo '["Text"]' | python3 scripts/embed.py --stdin --model sentence-transformers/all-mpnet-base-v2

# From arguments
python3 scripts/embed.py "Text 1" "Text 2"
```

**Output Format:**
```json
{
  "embeddings": [[0.123, -0.456, ...]],
  "model": "sentence-transformers/all-MiniLM-L6-v2",
  "dimension": 384,
  "count": 1
}
```

**Memory Usage:**
- all-MiniLM-L6-v2: ~200MB
- all-mpnet-base-v2: ~600MB

## Integration with gh-star-search

These scripts are called automatically by gh-star-search when:
- Python 3 is available in PATH
- The scripts are executable
- Dependencies are installed (for enhanced methods)

The application gracefully falls back if:
- Python is not available (skips summarization/embeddings)
- Dependencies are missing (uses heuristic methods)
- Execution fails (logs warning and continues)

## Configuration

Configure via `~/.config/gh-star-search/config.json`:

```json
{
  "embedding": {
    "provider": "local",
    "model": "sentence-transformers/all-MiniLM-L6-v2",
    "dimensions": 384,
    "enabled": true
  }
}
```

Or via environment variables:
```bash
export GH_STAR_SEARCH_EMBEDDING_ENABLED=true
export GH_STAR_SEARCH_EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2
```

## Performance

### Summarization Benchmarks

| Method | Model | RAM Usage | Speed | Quality |
|--------|-------|-----------|-------|---------|
| Heuristic | N/A | 10MB | Instant | Good |
| Transformers | DistilBART | 1.5GB | 1-2s | Excellent |

### Embedding Benchmarks

| Model | Dimensions | RAM Usage | Speed (10 repos) | Quality |
|-------|------------|-----------|------------------|---------|
| MiniLM-L6-v2 | 384 | 200MB | 0.5s | Good |
| MPNet-base-v2 | 768 | 600MB | 1.5s | Excellent |

## Troubleshooting

### "python not found in PATH"

Install Python 3:
```bash
# Ubuntu/Debian
sudo apt install python3

# macOS
brew install python3

# Windows
# Download from python.org
```

### "sentence-transformers not installed"

Install the package:
```bash
pip install sentence-transformers
```

### "transformers not installed"

Install the package:
```bash
pip install transformers torch
```

### Out of Memory Errors

If you encounter OOM errors:

1. **For summarization**: Use `--method heuristic` to avoid loading models
2. **For embeddings**: Use a smaller model or reduce batch size
3. **System-wide**: Increase swap space or use a machine with more RAM

### Script Permission Errors

Make scripts executable:
```bash
chmod +x scripts/summarize.py scripts/embed.py
```

## Development

### Testing Summarization

```bash
# Test heuristic method
echo "This is a test document with multiple sentences. It contains important information. Some sentences are more relevant than others. The algorithm should pick the best ones." | python3 scripts/summarize.py --method heuristic --max-sentences 2

# Test transformers method
echo "Long document..." | python3 scripts/summarize.py --method transformers
```

### Testing Embeddings

```bash
# Test basic embedding
echo '["artificial intelligence", "machine learning"]' | python3 scripts/embed.py --stdin

# Test similarity
python3 -c "
import json
import sys
from scripts.embed import generate_embeddings
import numpy as np

texts = ['cat', 'dog', 'computer']
embeddings = generate_embeddings(texts)

# Calculate cosine similarity
for i, t1 in enumerate(texts):
    for j, t2 in enumerate(texts):
        if i < j:
            sim = np.dot(embeddings[i], embeddings[j])
            print(f'{t1} <-> {t2}: {sim:.3f}')
"
```

## License

These scripts are part of gh-star-search and are licensed under the same terms (MIT License).
