// Package notify sends one email per activation by executing the
// "signup-notify" Application Integration in the region project.
//
// Why Application Integration and not an alert policy or an external mail
// provider: GCP has no general-purpose email API. The Monitoring email
// channel is credential-free but welded to incident semantics (fixed
// subject, unavoidable resolved emails); an external provider needs an
// account and a secret. Application Integration's Send Email task is the
// one native, credential-free, per-event sender: one execution = one email
// with a custom subject and body. See the design decision
// (identity/marketplace-signup-interim-bootstrap.md).
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// defaultBaseURL is the Application Integration API host.
	defaultBaseURL = "https://integrations.googleapis.com"

	// triggerID is the API trigger defined in the signup-notify integration;
	// it is fixed by the integration definition, not configurable here.
	triggerID = "api_trigger/SignupNotify_API_1"

	cloudPlatformURL = "https://www.googleapis.com/auth/cloud-platform"
)

// Client executes the signup-notify integration. It is a separate client
// from the workflow one because the failure semantics differ: a failed
// notification must never affect an activation that already succeeded.
type Client struct {
	ProjectID   string
	Region      string
	Integration string

	// HTTP is the authenticated client. BaseURL exists for tests.
	HTTP    *http.Client
	BaseURL string

	// Timeout bounds a single execute call. Notification is fire-and-forget;
	// the caller discards the error after logging it.
	Timeout time.Duration
}

// NewClient builds a client using Application Default Credentials, the same
// identity that invokes the activate-project workflow.
func NewClient(ctx context.Context, projectID, region, integration string) (*Client, error) {
	ts, err := google.DefaultTokenSource(ctx, cloudPlatformURL)
	if err != nil {
		return nil, fmt.Errorf("resolve default credentials: %w", err)
	}
	return &Client{
		ProjectID:   projectID,
		Region:      region,
		Integration: integration,
		HTTP:        oauth2.NewClient(ctx, ts),
		BaseURL:     defaultBaseURL,
		Timeout:     15 * time.Second,
	}, nil
}

// Notify executes the integration with the activation facts. The recipient
// is fixed in the integration definition — Signer, Project, and Status are
// the only inputs the caller controls — so the backend cannot redirect mail.
func (c *Client) Notify(ctx context.Context, signer, project, status string) error {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	body, err := json.Marshal(executeRequest{
		TriggerID: triggerID,
		InputParameters: map[string]parameterValue{
			"Signer":  {StringValue: signer},
			"Project": {StringValue: project},
			"Status":  {StringValue: status},
		},
	})
	if err != nil {
		return fmt.Errorf("encode notification request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/projects/%s/locations/%s/integrations/%s:execute",
		c.BaseURL, c.ProjectID, c.Region, c.Integration)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("execute integration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("execute integration: HTTP %d: %s", resp.StatusCode, snippet)
	}

	// The response body is large execution metadata; drain a bounded amount
	// so the connection can be reused, then discard.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return nil
}

// executeRequest is the subset of the integrations.execute request.
type executeRequest struct {
	TriggerID       string                    `json:"triggerId"`
	InputParameters map[string]parameterValue `json:"inputParameters"`
}

type parameterValue struct {
	StringValue string `json:"stringValue"`
}
