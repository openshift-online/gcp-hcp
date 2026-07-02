package hyperfleetapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
)

const (
	defaultTimeout     = 10 * time.Second
	maxRetryAttempts   = 3
	initialBackoff     = 1 * time.Second
)

// NotFoundError is returned when the server responds with HTTP 404.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("resource %s/%s not found", e.Resource, e.ID)
}

// Client defines the HyperFleet API operations used by adapters.
type Client interface {
	GetCluster(ctx context.Context, clusterID string) (*ClusterDetail, error)
	GetClusterStatuses(ctx context.Context, clusterID string) (AdapterStatuses, error)
	PutClusterStatus(ctx context.Context, clusterID string, payload StatusPayload) error

	GetNodePool(ctx context.Context, nodepoolID string) (*NodePoolDetail, error)
	GetNodePoolStatuses(ctx context.Context, nodepoolID string) (AdapterStatuses, error)
	PutNodePoolStatus(ctx context.Context, nodepoolID string, payload StatusPayload) error
}

// HTTPClient is the standard-library-based implementation of Client.
type HTTPClient struct {
	baseURL    string
	apiVersion string
	httpClient *http.Client
	log        logger.Logger
}

// New creates a new HTTPClient.
// baseURL should not have a trailing slash (e.g., "https://api.example.com").
// apiVersion is the API path segment (e.g., "v1").
func New(baseURL, apiVersion string, log logger.Logger) *HTTPClient {
	return &HTTPClient{
		baseURL:    baseURL,
		apiVersion: apiVersion,
		httpClient: &http.Client{Timeout: defaultTimeout},
		log:        log,
	}
}

// url builds the full URL for a given path (e.g., "/clusters/abc").
func (c *HTTPClient) url(path string) string {
	return fmt.Sprintf("%s/api/hyperfleet/%s%s", c.baseURL, c.apiVersion, path)
}

// get performs a GET request with retry on 5xx, unmarshal into dest.
func (c *HTTPClient) get(ctx context.Context, path string, dest interface{}) error {
	return c.doWithRetry(ctx, http.MethodGet, path, nil, dest)
}

// put performs a PUT request with retry on 5xx.
func (c *HTTPClient) put(ctx context.Context, path string, body interface{}) error {
	return c.doWithRetry(ctx, http.MethodPut, path, body, nil)
}

// doWithRetry executes an HTTP request, retrying on 5xx with exponential backoff.
// Returns NotFoundError on 404. Body is JSON-encoded if non-nil.
func (c *HTTPClient) doWithRetry(ctx context.Context, method, path string, body interface{}, dest interface{}) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("hyperfleetapi: marshal request body: %w", err)
		}
	}

	fullURL := c.url(path)
	backoff := initialBackoff
	var lastErr error

	for attempt := 1; attempt <= maxRetryAttempts; attempt++ {
		c.log.Debugf(ctx, "hyperfleetapi: %s %s (attempt %d/%d)", method, fullURL, attempt, maxRetryAttempts)

		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
		if err != nil {
			return fmt.Errorf("hyperfleetapi: build request: %w", err)
		}
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("hyperfleetapi: %s %s: %w", method, fullURL, err)
			c.log.Errorf(ctx, "hyperfleetapi: request error (attempt %d): %v", attempt, lastErr)
			if attempt < maxRetryAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= 2
			}
			continue
		}

		respBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck

		if readErr != nil {
			lastErr = fmt.Errorf("hyperfleetapi: read response body: %w", readErr)
			c.log.Errorf(ctx, "hyperfleetapi: read body error (attempt %d): %v", attempt, lastErr)
			if attempt < maxRetryAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= 2
			}
			continue
		}

		// 404 — not found, no retry
		if resp.StatusCode == http.StatusNotFound {
			return &NotFoundError{Resource: path}
		}

		// 5xx — retry
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("hyperfleetapi: %s %s returned %d: %s", method, fullURL, resp.StatusCode, string(respBytes))
			c.log.Errorf(ctx, "hyperfleetapi: server error (attempt %d): %v", attempt, lastErr)
			if attempt < maxRetryAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= 2
			}
			continue
		}

		// Other non-2xx errors — no retry
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("hyperfleetapi: %s %s returned %d: %s", method, fullURL, resp.StatusCode, string(respBytes))
		}

		// Success: unmarshal if dest provided
		if dest != nil && len(respBytes) > 0 {
			if err := json.Unmarshal(respBytes, dest); err != nil {
				return fmt.Errorf("hyperfleetapi: unmarshal response: %w", err)
			}
		}
		return nil
	}

	return fmt.Errorf("hyperfleetapi: %s %s failed after %d attempts: %w", method, fullURL, maxRetryAttempts, lastErr)
}

// GetCluster fetches a cluster by ID.
func (c *HTTPClient) GetCluster(ctx context.Context, clusterID string) (*ClusterDetail, error) {
	var cluster ClusterDetail
	if err := c.get(ctx, fmt.Sprintf("/clusters/%s", clusterID), &cluster); err != nil {
		return nil, err
	}
	return &cluster, nil
}

// GetClusterStatuses fetches all adapter statuses for a cluster.
func (c *HTTPClient) GetClusterStatuses(ctx context.Context, clusterID string) (AdapterStatuses, error) {
	var page struct {
		Items AdapterStatuses `json:"items"`
	}
	if err := c.get(ctx, fmt.Sprintf("/clusters/%s/statuses", clusterID), &page); err != nil {
		return nil, err
	}
	return page.Items, nil
}

// PutClusterStatus updates the adapter status for a cluster.
func (c *HTTPClient) PutClusterStatus(ctx context.Context, clusterID string, payload StatusPayload) error {
	return c.put(ctx, fmt.Sprintf("/clusters/%s/statuses", clusterID), payload)
}

// GetNodePool fetches a node pool by ID.
func (c *HTTPClient) GetNodePool(ctx context.Context, nodepoolID string) (*NodePoolDetail, error) {
	var np NodePoolDetail
	if err := c.get(ctx, fmt.Sprintf("/nodepools/%s", nodepoolID), &np); err != nil {
		return nil, err
	}
	return &np, nil
}

// GetNodePoolStatuses fetches all adapter statuses for a node pool.
func (c *HTTPClient) GetNodePoolStatuses(ctx context.Context, nodepoolID string) (AdapterStatuses, error) {
	var page struct {
		Items AdapterStatuses `json:"items"`
	}
	if err := c.get(ctx, fmt.Sprintf("/nodepools/%s/statuses", nodepoolID), &page); err != nil {
		return nil, err
	}
	return page.Items, nil
}

// PutNodePoolStatus updates the adapter status for a node pool.
func (c *HTTPClient) PutNodePoolStatus(ctx context.Context, nodepoolID string, payload StatusPayload) error {
	return c.put(ctx, fmt.Sprintf("/nodepools/%s/statuses", nodepoolID), payload)
}

// Ensure HTTPClient implements Client.
var _ Client = (*HTTPClient)(nil)
