// Package config loads and validates the backend's configuration from the
// environment. Every value is required unless a documented default applies;
// nothing has a hardcoded fallback for identity-bearing settings.
package config

import (
	"fmt"
	"strings"
)

// Config is the backend's runtime configuration.
type Config struct {
	// Port is the HTTP listen port. Cloud Run sets PORT.
	Port string

	// OAuthClientID is the dedicated OAuth client the SPA signs in with.
	// ID tokens must carry it in their aud claim.
	OAuthClientID string

	// AllowedEmailDomains is the hosted-domain allowlist. Google's signed
	// hd claim must equal one of these, and the email domain must agree.
	AllowedEmailDomains []string

	// Workflow coordinates for the existing activate-project workflow.
	WorkflowProjectID string
	WorkflowRegion    string
	WorkflowName      string

	// FrontendDir, when set, serves the SPA from disk at the same origin.
	// Local development only: in deployment the SPA comes from GCS + CDN.
	FrontendDir string

	// AllowedOrigin is the SPA origin permitted by CORS. In deployment the SPA
	// (GCS + CDN) and the API (GKE Gateway) have different hostnames. Empty
	// means no cross-origin access: only same-origin requests work.
	AllowedOrigin string
}

// Getenv mirrors os.Getenv so Load is testable.
type Getenv func(string) string

// Load reads configuration, returning an error naming every missing or
// malformed required value.
func Load(getenv Getenv) (Config, error) {
	cfg := Config{
		Port:              orDefault(getenv("PORT"), "8080"),
		OAuthClientID:     strings.TrimSpace(getenv("OAUTH_CLIENT_ID")),
		WorkflowProjectID: strings.TrimSpace(getenv("WORKFLOW_PROJECT_ID")),
		WorkflowRegion:    strings.TrimSpace(getenv("WORKFLOW_REGION")),
		WorkflowName:      orDefault(strings.TrimSpace(getenv("WORKFLOW_NAME")), "activate-project"),
		FrontendDir:       strings.TrimSpace(getenv("FRONTEND_DIR")),
		AllowedOrigin:     strings.TrimRight(strings.TrimSpace(getenv("ALLOWED_ORIGIN")), "/"),
	}

	cfg.AllowedEmailDomains = splitList(orDefault(getenv("ALLOWED_EMAIL_DOMAINS"), "redhat.com"))

	var missing []string
	if cfg.OAuthClientID == "" {
		missing = append(missing, "OAUTH_CLIENT_ID")
	}
	if cfg.WorkflowProjectID == "" {
		missing = append(missing, "WORKFLOW_PROJECT_ID")
	}
	if cfg.WorkflowRegion == "" {
		missing = append(missing, "WORKFLOW_REGION")
	}
	if len(cfg.AllowedEmailDomains) == 0 {
		missing = append(missing, "ALLOWED_EMAIL_DOMAINS")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func orDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func splitList(value string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, part := range strings.Split(value, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		if _, dup := seen[part]; dup {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}
