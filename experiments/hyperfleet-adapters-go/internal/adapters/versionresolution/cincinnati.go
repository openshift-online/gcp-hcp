package versionresolution

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultCincinnatiBaseURL = "https://api.openshift.com/api/upgrades_info/v1/graph"

// CincinnatiClient is an HTTP client for the Cincinnati update graph API.
type CincinnatiClient struct {
	BaseURL    string
	Arch       string
	httpClient *http.Client
}

// NewCincinnatiClient creates a new CincinnatiClient with a 30s timeout.
func NewCincinnatiClient(baseURL, arch string) *CincinnatiClient {
	if baseURL == "" {
		baseURL = defaultCincinnatiBaseURL
	}
	return &CincinnatiClient{
		BaseURL: baseURL,
		Arch:    arch,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ReleaseInfo is one node in the Cincinnati graph.
type ReleaseInfo struct {
	Version string `json:"version"`
	Payload string `json:"payload"`
}

// CincinnatiGraph is the raw response from Cincinnati.
type CincinnatiGraph struct {
	Nodes []ReleaseInfo `json:"nodes"`
}

// Resolve fetches the Cincinnati graph for the given channel and returns the
// ReleaseInfo matching version. Returns nil, nil if the version is not found.
func (c *CincinnatiClient) Resolve(ctx context.Context, version, channel string) (*ReleaseInfo, error) {
	url := fmt.Sprintf("%s?channel=%s&arch=%s", c.BaseURL, channel, c.Arch)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cincinnati: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cincinnati: GET %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cincinnati: read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cincinnati: GET %s returned %d: %s", url, resp.StatusCode, string(body))
	}

	var graph CincinnatiGraph
	if err := json.Unmarshal(body, &graph); err != nil {
		return nil, fmt.Errorf("cincinnati: unmarshal response: %w", err)
	}

	for i := range graph.Nodes {
		if graph.Nodes[i].Version == version {
			return &graph.Nodes[i], nil
		}
	}

	// Version not found — not an error, caller handles requeue.
	return nil, nil
}
