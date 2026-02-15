# Evaluation Improvement Plan for Less Popular Repositories

## Problem Statement

Current evaluation yields all 0.000 metrics because:
1. Test queries expect mega-popular repos (ripgrep, django, flask) that don't exist in the database
2. User's 26-repo database represents personal/niche projects with star distribution:
   - 0-99 stars: 6 repos (avg 43 ⭐)
   - 100-499 stars: 5 repos (avg 224 ⭐)
   - 500-999 stars: 4 repos (avg 635 ⭐)
   - 1k-5k stars: 8 repos (avg 1,849 ⭐)
   - 5k+ stars: 3 repos (avg 54,811 ⭐, including openclaw at 141k)

## Current Ranking Algorithm Analysis

From `evaluate_embeddings.py` line 194-211:

```python
def _apply_ranking_boosts(results: list[dict]) -> list[dict]:
    for result in results:
        base_score = result["base_score"]  # Cosine similarity

        # Star boost: 1.0 + (0.1 * log10(stars + 1) / 6.0)
        star_boost = 1.0 + (0.1 * math.log10(result["stargazers_count"] + 1) / 6.0)

        # Recency: 1.0 - 0.2 * min(1.0, days_since_update / 365)
        days_since_update = (datetime.now(updated_at.tzinfo) - updated_at).days
        recency_factor = 1.0 - 0.2 * min(1.0, days_since_update / 365.0)

        result["final_score"] = base_score * star_boost * recency_factor
```

### Issues with Current Algorithm

**Star Boost Impact:**
- 10 stars: `1.0 + (0.1 * log10(11) / 6.0)` = 1.017 (1.7% boost)
- 100 stars: `1.0 + (0.1 * log10(101) / 6.0)` = 1.033 (3.3% boost)
- 1,000 stars: `1.0 + (0.1 * log10(1001) / 6.0)` = 1.050 (5.0% boost)
- 10,000 stars: `1.0 + (0.1 * log10(10001) / 6.0)` = 1.067 (6.7% boost)
- 100,000 stars: `1.0 + (0.1 * log10(100001) / 6.0)` = 1.083 (8.3% boost)

**Problem**: Star boost heavily favors popularity over relevance. A less popular but highly relevant repo can be outranked by a popular but marginally relevant repo.

**Example:**
- Repo A: 100 stars, cosine similarity 0.95 → final score: 0.95 * 1.033 = 0.981
- Repo B: 10,000 stars, cosine similarity 0.85 → final score: 0.85 * 1.067 = 0.907

In this case, the more relevant but less popular repo wins. But:

- Repo A: 100 stars, cosine similarity 0.85 → final score: 0.85 * 1.033 = 0.878
- Repo B: 10,000 stars, cosine similarity 0.85 → final score: 0.85 * 1.067 = 0.907

With equal relevance, popularity dominates.

## Proposed Solutions

### Option 1: Reduce Star Boost Magnitude (Conservative)

**Change:** Reduce star boost coefficient from 0.1 to 0.05

```python
star_boost = 1.0 + (0.05 * math.log10(result["stargazers_count"] + 1) / 6.0)
```

**Impact:**
- 10 stars: 1.008 (0.8% boost, was 1.7%)
- 1,000 stars: 1.025 (2.5% boost, was 5.0%)
- 100,000 stars: 1.042 (4.2% boost, was 8.3%)

**Pros:**
- Simple change
- Still rewards popularity but less aggressively
- Relevance (cosine similarity) becomes more dominant

**Cons:**
- Doesn't fundamentally address the bias

### Option 2: Adaptive Star Boost (Context-Aware)

**Change:** Scale star boost based on database star distribution

```python
def _compute_adaptive_star_boost(stars: int, percentile_rank: float) -> float:
    """
    Use percentile rank within database instead of absolute star count.

    percentile_rank: 0.0-1.0, where 1.0 = highest starred repo in DB
    """
    # Boost ranges from 1.0 (0th percentile) to 1.05 (100th percentile)
    return 1.0 + (0.05 * percentile_rank)
```

**Implementation:**
```python
# Pre-compute percentile ranks
star_counts = [r["stargazers_count"] for r in all_repos]
for result in results:
    percentile = percentileofscore(star_counts, result["stargazers_count"]) / 100
    star_boost = 1.0 + (0.05 * percentile)
```

**Pros:**
- Fair comparison within database context
- 141k-star repo and 100-star repo both get meaningful differentiation
- Adapts automatically to database composition

**Cons:**
- More complex
- Requires access to all repos during ranking
- Percentile can change as database grows

### Option 3: Make Star Boost Optional/Configurable

**Change:** Add evaluation mode that disables ranking boosts

```python
def evaluate_model(self, model_config: ModelConfig, use_boosts: bool = True):
    # ... existing code ...
    if use_boosts:
        results = _apply_ranking_boosts(results)
```

**Pros:**
- Can compare "pure semantic similarity" vs "boosted results"
- Useful for understanding boost impact
- Simple to implement

**Cons:**
- Doesn't fix the core issue
- May diverge from production behavior

### Recommendation: Hybrid Approach

Combine Options 1 and 3:
1. Reduce star boost coefficient (0.1 → 0.05)
2. Add `--pure-semantic` flag to disable boosts for baseline comparison
3. Document the ranking formula clearly in evaluation output

## Test Query Generation Strategy

### Approach A: Use Top-N Most Starred Repos (Recommended)

**Rationale:** Most starred repos likely represent your core interests and will have richer metadata (descriptions, topics, contributors).

**Implementation:**

```python
# Generate queries from top 50-100 repos
def generate_queries_from_top_repos(db_path: str, n: int = 50) -> list[dict]:
    """Generate test queries from top N most-starred repos."""
    db = duckdb.connect(db_path)
    repos = db.execute(f"""
        SELECT full_name, description, purpose, topics_text, language
        FROM repositories
        ORDER BY stargazers_count DESC
        LIMIT {n}
    """).fetchall()

    queries = []

    for repo in repos:
        full_name, desc, purpose, topics, lang = repo

        # Query 1: Direct repo name search
        queries.append({
            "id": f"direct_{full_name.replace('/', '_')}",
            "query": full_name.split('/')[1],  # Just repo name
            "category": "direct_search",
            "expected_repos": [full_name],
            "relevance_grades": {full_name: 3}
        })

        # Query 2: Topic-based search (if has topics)
        if topics:
            topic_list = topics.split()[:3]  # First 3 topics
            queries.append({
                "id": f"topic_{full_name.replace('/', '_')}",
                "query": " ".join(topic_list),
                "category": "topic_based",
                "expected_repos": [full_name],
                "relevance_grades": {full_name: 3}
            })

        # Query 3: Language + key term search
        if lang and (purpose or desc):
            key_term = (purpose or desc).split()[0:3]  # First few words
            queries.append({
                "id": f"lang_{full_name.replace('/', '_')}",
                "query": f"{lang} {' '.join(key_term)}",
                "category": "language_specific",
                "expected_repos": [full_name],
                "relevance_grades": {full_name: 3}
            })

    return queries
```

**Pros:**
- Queries guaranteed to have matches in database
- Reflects actual user interests (starred repos)
- Can generate many query variations per repo
- Stable over time (top repos rarely change drastically)

**Cons:**
- Queries are somewhat artificial (extracted from metadata)
- May not reflect natural search patterns
- Biased toward repos with good metadata

### Approach B: Curate Real Search Scenarios

**Rationale:** Hand-craft queries based on actual use cases for the repos in database.

**Example queries from current database:**

```toml
# Developer tools
[[queries]]
id = "q001"
query = "go style guide"
category = "documentation"
expected_repos = ["uber-go/guide"]
relevance_grades = { "uber-go/guide" = 3 }

[[queries]]
id = "q002"
query = "macos ai assistant"
category = "use_case"
expected_repos = ["openclaw/openclaw"]
relevance_grades = { "openclaw/openclaw" = 3 }

[[queries]]
id = "q003"
query = "google cli tool"
category = "composite"
expected_repos = ["steipete/gogcli"]
relevance_grades = { "steipete/gogcli" = 3 }

[[queries]]
id = "q004"
query = "swift menu bar app"
category = "composite"
expected_repos = ["steipete/CodexBar", "steipete/Peekaboo"]
relevance_grades = { "steipete/CodexBar" = 3, "steipete/Peekaboo" = 2 }

[[queries]]
id = "q005"
query = "network discovery tool"
category = "use_case"
expected_repos = ["ramonvermeulen/whosthere"]
relevance_grades = { "ramonvermeulen/whosthere" = 3 }

[[queries]]
id = "q006"
query = "aws iam policy"
category = "use_case"
expected_repos = ["salesforce/policy_sentry"]
relevance_grades = { "salesforce/policy_sentry" = 3 }

[[queries]]
id = "q007"
query = "text to speech python"
category = "composite"
expected_repos = ["Blaizzy/mlx-audio"]
relevance_grades = { "Blaizzy/mlx-audio" = 3 }

# Generic searches that should match multiple repos
[[queries]]
id = "q008"
query = "cli tool"
category = "generic"
expected_repos = ["steipete/gogcli", "ramonvermeulen/whosthere", "steipete/summarize"]
relevance_grades = {
    "steipete/gogcli" = 2,
    "ramonvermeulen/whosthere" = 2,
    "steipete/summarize" = 2
}

[[queries]]
id = "q009"
query = "swift macos"
category = "technology"
expected_repos = ["steipete/CodexBar", "steipete/Peekaboo", "steipete/AXorcist"]
relevance_grades = {
    "steipete/CodexBar" = 2,
    "steipete/Peekaboo" = 2,
    "steipete/AXorcist" = 2
}

[[queries]]
id = "q010"
query = "ai assistant"
category = "use_case"
expected_repos = ["openclaw/openclaw"]
relevance_grades = { "openclaw/openclaw" = 3 }
```

**Pros:**
- Natural search patterns
- Reflects real user intent
- Can test both specific and broad queries
- High-quality ground truth

**Cons:**
- Manual curation required
- Takes time to create 20-30 queries
- Needs updating when repos change

### Approach C: Hybrid (Recommended)

1. **Auto-generate baseline queries** (Approach A) for coverage
2. **Hand-curate 10-15 key queries** (Approach B) for quality
3. **Include cross-cutting queries** that match multiple repos

**Implementation script:**

```bash
# Step 1: Auto-generate candidate queries
uv run python scripts/generate_eval_queries.py \
    --db ~/.config/gh-star-search/database.db \
    --top-n 50 \
    --output eval_config_generated.toml

# Step 2: Review and manually refine
# - Remove redundant queries
# - Add cross-cutting queries
# - Adjust relevance grades
# - Add queries for edge cases

# Step 3: Validate queries
uv run python evaluate_embeddings.py \
    --config eval_config_personal.toml \
    --db ~/.config/gh-star-search/database.db
```

## Implementation Plan

### Phase 1: Fix Ranking Algorithm (1-2 hours)

1. **Reduce star boost coefficient**
   - Change `0.1` to `0.05` in `_apply_ranking_boosts()`
   - Add comment explaining the rationale

2. **Add evaluation mode flag**
   - Add `--pure-semantic` flag to disable ranking boosts
   - Update help text and documentation

3. **Test with current queries**
   - Run evaluation with both modes
   - Document the difference in results

4. **Update documentation**
   - Add ranking formula explanation to `EVALUATION.md`
   - Include boost impact table for various star counts

### Phase 2: Generate Personal Test Queries (2-3 hours)

1. **Create query generation script**
   - File: `generate_eval_queries.py`
   - Implement Approach A (auto-generation from top repos)
   - Output to `eval_config_generated.toml`

2. **Manual curation**
   - Review generated queries
   - Add 10-15 hand-crafted queries for key use cases
   - Add cross-cutting queries
   - Save as `eval_config_personal.toml`

3. **Run initial evaluation**
   - Validate queries return expected repos
   - Check metric distribution (should no longer be all 0.000)
   - Adjust relevance grades based on actual results

### Phase 3: Establish Baseline (1 hour)

1. **Document current performance**
   - Run evaluation with personal queries
   - Save baseline metrics in `eval_results/baseline_personal.md`
   - Include both pure semantic and boosted rankings

2. **Create reference documentation**
   - Document expected MRR/Precision@k ranges for personal database
   - Add guidelines for when to update test queries
   - Note that metrics will differ from public benchmark expectations

### Phase 4: Sync More Repos (Optional)

If you want to test with a larger dataset:

1. **Sync full GitHub stars**
   ```bash
   ./gh-star-search sync --force
   ```

2. **Re-run evaluation**
   - Use both original queries (popular repos) and personal queries
   - Compare results across different database sizes
   - Document how evaluation scales

## Success Criteria

- [ ] Evaluation produces non-zero metrics (MRR > 0.0)
- [ ] Test queries return expected repos in top 10 results
- [ ] Ranking boost impact is documented and understood
- [ ] Personal query set covers diverse repo types
- [ ] Baseline performance documented for future comparisons
- [ ] Evaluation runs in < 5 minutes (with caching)

## Open Questions

1. **Should we keep original queries?**
   - Keep them in `eval_config_public.toml` for reference
   - Use `eval_config_personal.toml` for actual evaluation

2. **What's the target dataset size?**
   - Current: 26 repos
   - Recommended: 100-500 repos for meaningful evaluation
   - Full stars: Potentially thousands

3. **How often to update test queries?**
   - After significant repo additions (>10% growth)
   - When new categories/topics emerge
   - Quarterly review recommended

## Next Steps

**Immediate (this session):**
1. Reduce star boost coefficient (quick fix)
2. Create 10 hand-crafted test queries for current 26 repos
3. Run evaluation to validate non-zero metrics

**Short-term (next session):**
1. Implement query generation script
2. Build comprehensive personal query set
3. Document baseline performance

**Long-term (future):**
1. Sync more repos (target: 100-500)
2. Implement adaptive star boost (Option 2)
3. Add category-specific metric breakdowns
4. Consider A/B testing different ranking formulas
