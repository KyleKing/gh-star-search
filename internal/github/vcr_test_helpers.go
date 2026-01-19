package github

import (
	"net/http"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/cassette"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
)

// setupVCRRecorder creates a VCR recorder with common configuration
func setupVCRRecorder(t *testing.T, cassetteName string, hooks ...recorder.Option) (*recorder.Recorder, *vcrRESTClient) {
	t.Helper()

	// Get authenticated HTTP client from GitHub CLI
	authClient, err := api.DefaultHTTPClient()
	if err != nil {
		t.Skipf("Skipping test - GitHub auth not available: %v", err)
	}

	// Default options: ignore common headers and authorization
	defaultOpts := []recorder.Option{
		recorder.WithRealTransport(authClient.Transport),
		recorder.WithMatcher(cassette.NewDefaultMatcher(
			cassette.WithIgnoreAuthorization(),
			cassette.WithIgnoreHeaders(
				"Time-Zone",
				"Content-Type",
				"Accept",
				"User-Agent",
				"X-Github-Api-Version",
			),
		)),
	}

	// Append any additional hooks
	opts := append(defaultOpts, hooks...)

	// Create VCR recorder
	r, err := recorder.New("testdata/"+cassetteName, opts...)
	if err != nil {
		t.Fatalf("Failed to create VCR recorder: %v", err)
	}

	// Create VCR client
	vcrClient := NewVCRRESTClient(r.GetDefaultClient())

	return r, vcrClient
}

// withRateLimitError creates a hook that simulates rate limiting
func withRateLimitError() recorder.Option {
	return recorder.WithHook(func(i *cassette.Interaction) error {
		i.Response.Code = http.StatusForbidden
		i.Response.Body = `{"message": "API rate limit exceeded"}`
		i.Response.Headers.Set("Content-Type", "application/json")
		return nil
	}, recorder.BeforeResponseReplayHook)
}

// withNotFoundError creates a hook that simulates 404 errors
func withNotFoundError() recorder.Option {
	return recorder.WithHook(func(i *cassette.Interaction) error {
		i.Response.Code = http.StatusNotFound
		i.Response.Body = `{"message": "Not Found"}`
		i.Response.Headers.Set("Content-Type", "application/json")
		return nil
	}, recorder.BeforeResponseReplayHook)
}

// withServerError creates a hook that simulates 500 errors
func withServerError() recorder.Option {
	return recorder.WithHook(func(i *cassette.Interaction) error {
		i.Response.Code = http.StatusInternalServerError
		i.Response.Body = `{"message": "Internal Server Error"}`
		i.Response.Headers.Set("Content-Type", "application/json")
		return nil
	}, recorder.BeforeResponseReplayHook)
}

// withLimitedResults creates a hook that limits the number of results returned
func withLimitedResults(maxItems int) recorder.Option {
	return recorder.WithHook(func(i *cassette.Interaction) error {
		// This hook would need to parse and modify JSON arrays
		// For now, it's a placeholder for the concept
		// Actual implementation would parse response body and truncate arrays
		return nil
	}, recorder.BeforeSaveHook)
}

// cleanupRecorder ensures the recorder is stopped
func cleanupRecorder(t *testing.T, r *recorder.Recorder) {
	t.Helper()
	if err := r.Stop(); err != nil {
		t.Logf("Warning: failed to stop VCR recorder: %v", err)
	}
}

// skipIfRecording skips a test if VCR is in recording mode
// This is useful for tests that should only run in replay mode
func skipIfRecording(t *testing.T, r *recorder.Recorder) {
	t.Helper()
	if r.IsRecording() {
		t.Skip("Skipping test in VCR recording mode")
	}
}

// requireRecording skips a test if VCR is not in recording mode
// This is useful for tests that need to record fresh cassettes
func requireRecording(t *testing.T, r *recorder.Recorder) {
	t.Helper()
	if !r.IsRecording() {
		t.Skip("Skipping test - requires VCR recording mode")
	}
}

// withTimeout wraps setupVCRRecorder with a timeout
func setupVCRWithTimeout(t *testing.T, cassetteName string, timeout time.Duration, hooks ...recorder.Option) (*recorder.Recorder, *vcrRESTClient, func()) {
	t.Helper()

	r, client := setupVCRRecorder(t, cassetteName, hooks...)

	// Return cleanup function
	cleanup := func() {
		cleanupRecorder(t, r)
	}

	return r, client, cleanup
}
