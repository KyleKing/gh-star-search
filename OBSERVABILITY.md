# Observability Guide for gh-star-search

## Overview

This document provides guidance on implementing observability for gh-star-search, a CLI tool for searching and managing GitHub starred repositories. Effective observability enables monitoring of application performance, debugging issues, and understanding usage patterns.

## Philosophy

For a CLI application like gh-star-search, observability needs differ from traditional server applications:

- **Lightweight**: Minimize overhead since the tool runs on user machines
- **Local-first**: Focus on local logging and metrics rather than centralized collection
- **Privacy-conscious**: Avoid collecting sensitive user data
- **Opt-in**: Users should control what metrics are collected and where they're sent

## Key Metrics to Track

### 1. Command Execution Metrics

**Sync Command:**
- Total sync duration
- Number of repositories synced
- API requests made (count and rate)
- Rate limit remaining/used
- Content download sizes and durations
- Embedding generation time (when enabled)
- Summarization time (when enabled)
- Database write performance

**Search Command:**
- Query execution time
- Result count
- Search mode (fuzzy vs. vector)
- Query complexity (term count)
- Index size

**Stream Command:**
- Content fetch duration
- Number of files downloaded
- Total bytes downloaded
- Filtering efficiency (files filtered vs. retained)

### 2. Resource Utilization

- Memory usage (heap allocation, GC pressure)
- Database connection pool utilization
- Disk I/O operations
- CPU usage during embedding generation

### 3. Error Metrics

- API errors (rate limits, authentication, network)
- Database errors (connection, query failures)
- Embedding generation failures
- File system errors

### 4. Business Metrics

- Repository catalog size
- Embedding coverage percentage
- Summary coverage percentage
- Search result quality (implicit through user behavior)

## Implementation Approaches

### 1. Built-in Go Metrics (Recommended for CLI)

**Package: `expvar`**

Go's `expvar` package provides a standardized interface for exposing metrics via HTTP endpoint.

```go
import (
    "expvar"
    "time"
)

var (
    syncCount = expvar.NewInt("sync_total")
    syncDuration = expvar.NewFloat("sync_duration_seconds")
    searchCount = expvar.NewInt("search_total")
    apiErrors = expvar.NewInt("api_errors_total")
)

// In sync command
start := time.Now()
defer func() {
    syncCount.Add(1)
    syncDuration.Set(time.Since(start).Seconds())
}()
```

**Pros:**
- Built-in, no dependencies
- Minimal overhead
- Simple to implement

**Cons:**
- Requires HTTP server (not ideal for CLI)
- Limited metric types
- No built-in persistence

### 2. Structured Logging with Metrics

**Package: `log/slog`** (Go 1.21+)

```go
import (
    "log/slog"
    "os"
)

func setupLogger() *slog.Logger {
    opts := &slog.HandlerOptions{
        Level: slog.LevelInfo,
        AddSource: true,
    }

    handler := slog.NewJSONHandler(os.Stderr, opts)
    return slog.New(handler)
}

// Usage in code
logger.Info("sync completed",
    slog.Int("repositories_synced", count),
    slog.Duration("duration", elapsed),
    slog.Int("api_requests", apiCalls),
    slog.Float64("rate_limit_remaining", rateLimit),
)
```

**Pros:**
- Structured data suitable for analysis
- No external dependencies
- Can be parsed and aggregated later
- Privacy-friendly (local only)

**Cons:**
- Requires log aggregation for metrics
- Not real-time

### 3. Local Metrics File

Write metrics to a local JSON file that can be analyzed separately:

```go
type Metrics struct {
    Command    string        `json:"command"`
    StartTime  time.Time     `json:"start_time"`
    Duration   time.Duration `json:"duration"`
    Success    bool          `json:"success"`
    RepoCount  int           `json:"repo_count,omitempty"`
    ErrorMsg   string        `json:"error,omitempty"`
}

func recordMetrics(m Metrics) error {
    metricsPath := filepath.Join(os.UserHomeDir(), ".local", "share", "gh-star-search", "metrics.jsonl")

    f, err := os.OpenFile(metricsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    data, _ := json.Marshal(m)
    _, err = f.Write(append(data, '\n'))
    return err
}
```

**Pros:**
- Complete control over data format
- Privacy-friendly
- Can be analyzed with jq, Python, etc.
- Persistent history

**Cons:**
- Manual implementation
- Need to manage file rotation
- Not real-time

### 4. OpenTelemetry (For Advanced Use Cases)

**Package: `go.opentelemetry.io/otel`**

OpenTelemetry provides standardized observability (metrics, traces, logs):

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
    "go.opentelemetry.io/otel/sdk/metric"
)

func initMetrics() (metric.MeterProvider, error) {
    exporter := // Configure exporter (file, OTLP, etc.)

    provider := metric.NewMeterProvider(
        metric.WithReader(metric.NewPeriodicReader(exporter)),
    )

    otel.SetMeterProvider(provider)
    return provider, nil
}

// Usage
meter := otel.Meter("gh-star-search")
syncCounter, _ := meter.Int64Counter("sync.count")
syncCounter.Add(ctx, 1)
```

**Pros:**
- Industry standard
- Rich ecosystem
- Supports metrics, traces, and logs
- Flexible exporters

**Cons:**
- Heavy dependency
- Overkill for simple CLI
- Complexity

### 5. Prometheus Client (Traditional Approach)

**Package: `github.com/prometheus/client_golang/prometheus`**

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    syncDuration = promauto.NewHistogram(prometheus.HistogramOpts{
        Name: "gh_star_search_sync_duration_seconds",
        Help: "Duration of sync operations",
        Buckets: prometheus.DefBuckets,
    })

    searchCounter = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gh_star_search_queries_total",
            Help: "Total number of search queries",
        },
        []string{"mode"}, // fuzzy or vector
    )
)

// Usage
timer := prometheus.NewTimer(syncDuration)
defer timer.ObserveDuration()

searchCounter.WithLabelValues("vector").Inc()
```

**Pros:**
- Mature ecosystem
- Rich query language (PromQL)
- Excellent visualization with Grafana

**Cons:**
- Requires Prometheus server
- Not suitable for CLI without server mode
- Heavy infrastructure

## Recommended Approach for gh-star-search

**Hybrid approach combining structured logging and local metrics:**

1. **Use `log/slog` for operational logging** with structured fields
2. **Write summary metrics to local JSON file** for historical analysis
3. **Optional: Add `--metrics-port` flag** to expose expvar endpoint for development

### Implementation Example

```go
package metrics

import (
    "encoding/json"
    "log/slog"
    "os"
    "path/filepath"
    "time"
)

type MetricsRecorder struct {
    logger     *slog.Logger
    metricsDir string
}

func NewRecorder(logger *slog.Logger) (*MetricsRecorder, error) {
    homeDir, _ := os.UserHomeDir()
    metricsDir := filepath.Join(homeDir, ".local", "share", "gh-star-search")

    if err := os.MkdirAll(metricsDir, 0755); err != nil {
        return nil, err
    }

    return &MetricsRecorder{
        logger:     logger,
        metricsDir: metricsDir,
    }, nil
}

type CommandMetrics struct {
    Command   string                 `json:"command"`
    Timestamp time.Time              `json:"timestamp"`
    Duration  float64                `json:"duration_seconds"`
    Success   bool                   `json:"success"`
    Details   map[string]interface{} `json:"details,omitempty"`
    Error     string                 `json:"error,omitempty"`
}

func (r *MetricsRecorder) RecordCommand(m CommandMetrics) error {
    // Log to structured logger
    attrs := []slog.Attr{
        slog.String("command", m.Command),
        slog.Float64("duration", m.Duration),
        slog.Bool("success", m.Success),
    }

    if m.Error != "" {
        attrs = append(attrs, slog.String("error", m.Error))
    }

    r.logger.LogAttrs(nil, slog.LevelInfo, "command_executed", attrs...)

    // Write to metrics file
    metricsPath := filepath.Join(r.metricsDir, "metrics.jsonl")
    f, err := os.OpenFile(metricsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    data, _ := json.Marshal(m)
    _, err = f.Write(append(data, '\n'))

    return err
}

// Helper to create timer
func (r *MetricsRecorder) Timer(command string) func(success bool, details map[string]interface{}, err error) {
    start := time.Now()

    return func(success bool, details map[string]interface{}, err error) {
        m := CommandMetrics{
            Command:   command,
            Timestamp: start,
            Duration:  time.Since(start).Seconds(),
            Success:   success,
            Details:   details,
        }

        if err != nil {
            m.Error = err.Error()
        }

        _ = r.RecordCommand(m)
    }
}
```

### Usage in Commands

```go
// In cmd/sync.go
func runSync(ctx context.Context, cmd *cli.Command) error {
    recorder := getMetricsRecorder(ctx)
    done := recorder.Timer("sync")
    defer func() {
        done(err == nil, map[string]interface{}{
            "repo_count": count,
            "with_embed": withEmbed,
        }, err)
    }()

    // ... sync implementation
}
```

## Integration Points

### 1. Service Layer (internal/processor/service.go)

Add metrics for:
- Repository processing time
- Content download statistics
- File filtering efficiency

```go
type serviceImpl struct {
    // ... existing fields
    metrics *metrics.Recorder
}

func (s *serviceImpl) ProcessRepository(ctx context.Context, repoName string) error {
    start := time.Now()
    defer func() {
        s.metrics.RecordDuration("repository.process", time.Since(start))
    }()

    // ... implementation
}
```

### 2. Storage Layer (internal/storage/)

Add metrics for:
- Query execution time
- Connection pool usage
- Database size

```go
func (r *repositoryImpl) SearchRepositories(ctx context.Context, query string) ([]SearchResult, error) {
    timer := r.metrics.Timer("db.search")
    defer timer.Observe()

    // ... implementation
}
```

### 3. Embedding Generation (internal/embedding/)

Add metrics for:
- Embedding generation time
- Python subprocess execution time
- Success/failure rates

```go
func (p *LocalProviderImpl) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
    p.metrics.IncrementCounter("embedding.requests")
    start := time.Now()

    result, err := // ... generate embedding

    if err != nil {
        p.metrics.IncrementCounter("embedding.errors")
    } else {
        p.metrics.RecordDuration("embedding.generation", time.Since(start))
    }

    return result, err
}
```

### 4. CLI Commands

Each command should record:
- Execution time
- Success/failure
- Key parameters
- Result counts

## Analysis Tools

### Analyzing Metrics with jq

```bash
# Count commands by type
cat ~/.local/share/gh-star-search/metrics.jsonl | jq -s 'group_by(.command) | map({command: .[0].command, count: length})'

# Average sync duration
cat ~/.local/share/gh-star-search/metrics.jsonl | jq -s 'map(select(.command == "sync")) | map(.duration_seconds) | add / length'

# Recent errors
cat ~/.local/share/gh-star-search/metrics.jsonl | jq 'select(.success == false)'

# Sync performance over time
cat ~/.local/share/gh-star-search/metrics.jsonl | jq -s 'map(select(.command == "sync")) | map({timestamp, duration: .duration_seconds, repos: .details.repo_count})'
```

### Python Analysis Script

```python
import json
import pandas as pd
from pathlib import Path

metrics_path = Path.home() / ".local/share/gh-star-search/metrics.jsonl"

# Load metrics
metrics = []
with open(metrics_path) as f:
    for line in f:
        metrics.append(json.loads(line))

df = pd.DataFrame(metrics)
df['timestamp'] = pd.to_datetime(df['timestamp'])

# Analysis
print("Command execution summary:")
print(df.groupby('command').agg({
    'duration_seconds': ['count', 'mean', 'std', 'min', 'max'],
    'success': 'mean'
}))

# Plot sync performance over time
sync_df = df[df['command'] == 'sync']
sync_df.plot(x='timestamp', y='duration_seconds', kind='line')
```

## Configuration

Add configuration options to `internal/config/config.go`:

```go
type ObservabilityConfig struct {
    Enabled       bool   `json:"enabled"`        // Enable metrics collection
    MetricsPath   string `json:"metrics_path"`   // Path to metrics file
    LogLevel      string `json:"log_level"`      // trace, debug, info, warn, error
    LogFormat     string `json:"log_format"`     // json, text
    EnableExpvar  bool   `json:"enable_expvar"`  // Enable HTTP metrics endpoint
    ExpvarPort    int    `json:"expvar_port"`    // Port for expvar endpoint
}
```

## Privacy Considerations

When implementing observability:

1. **No PII**: Don't collect personally identifiable information
2. **No Repository Content**: Avoid logging repository file contents
3. **Local-First**: Default to local storage only
4. **Opt-in Remote**: If adding remote telemetry, make it explicitly opt-in
5. **Transparency**: Document what metrics are collected

## Performance Impact

Observability should have minimal impact on performance:

- **Logging**: Use async logging with buffered writers
- **Metrics**: Use atomic operations and lock-free counters
- **Sampling**: For high-frequency operations, consider sampling
- **Lazy Initialization**: Don't initialize metrics system if disabled

## Future Enhancements

1. **Dashboard**: Create simple TUI dashboard using `github.com/charmbracelet/bubbletea`
2. **Alerts**: Local alerting for issues (e.g., low disk space, rate limit exceeded)
3. **Export**: Add command to export metrics in various formats (CSV, JSON, Parquet)
4. **Aggregation**: Build summary statistics (daily/weekly/monthly)
5. **Profiling**: Integration with pprof for performance profiling

## References

- [Go log/slog package](https://pkg.go.dev/log/slog)
- [Go expvar package](https://pkg.go.dev/expvar)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
- [Prometheus Go client](https://github.com/prometheus/client_golang)
- [Effective Go - Logging](https://go.dev/blog/slog)

## Example: Complete Implementation Skeleton

See `internal/metrics/` package for a complete reference implementation following the recommended hybrid approach with structured logging and local metrics files.
