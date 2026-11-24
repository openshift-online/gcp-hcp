package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	env := map[string]string{
		"OAUTH_CLIENT_ID":     "123.apps.googleusercontent.com",
		"WORKFLOW_PROJECT_ID": "platform-project",
		"WORKFLOW_REGION":     "northamerica-northeast2",
	}
	cfg, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8080" || cfg.WorkflowName != "activate-project" {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
	if got, want := strings.Join(cfg.AllowedEmailDomains, ","), "redhat.com"; got != want {
		t.Fatalf("AllowedEmailDomains = %q, want %q", got, want)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	_, err := Load(func(string) string { return "" })
	if err == nil {
		t.Fatal("Load with empty environment succeeded, want error")
	}
	for _, want := range []string{"OAUTH_CLIENT_ID", "WORKFLOW_PROJECT_ID", "WORKFLOW_REGION"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not name %s", err.Error(), want)
		}
	}
}

func TestLoadNormalizesDomainList(t *testing.T) {
	env := map[string]string{
		"OAUTH_CLIENT_ID":       "c",
		"WORKFLOW_PROJECT_ID":   "p",
		"WORKFLOW_REGION":       "r",
		"ALLOWED_EMAIL_DOMAINS": " RedHat.com , redhat.com ,partners.example ",
	}
	cfg, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := strings.Join(cfg.AllowedEmailDomains, ","), "redhat.com,partners.example"; got != want {
		t.Fatalf("AllowedEmailDomains = %q, want %q", got, want)
	}
}
