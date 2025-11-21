package github

import (
	"encoding/json"
	"net/http"

	"github.com/cli/go-gh/v2/pkg/api"
)

// vcrRESTClient implements RESTClientInterface using VCR for HTTP recording/replay
type vcrRESTClient struct {
	httpClient *http.Client
}

// NewVCRRESTClient creates a new VCR REST client with the given HTTP client
func NewVCRRESTClient(httpClient *http.Client) *vcrRESTClient {
	return &vcrRESTClient{
		httpClient: httpClient,
	}
}

// Get implements RESTClientInterface.Get using VCR's HTTP client
func (v *vcrRESTClient) Get(path string, response interface{}) error {
	// Convert GitHub API path to full URL
	url := "https://api.github.com/" + path

	// Use the HTTP client's Get method (VCR will intercept the request)
	resp, err := v.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check for non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &api.HTTPError{StatusCode: resp.StatusCode}
	}

	// Decode the JSON response
	return json.NewDecoder(resp.Body).Decode(response)
}
