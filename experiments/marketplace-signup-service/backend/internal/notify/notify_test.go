package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testTimeout = 5 * time.Second

func TestNotifyPostsExecuteRequest(t *testing.T) {
	var gotPath string
	var gotBody executeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"executionId":"test"}`))
	}))
	defer srv.Close()

	c := &Client{
		ProjectID:   "proj",
		Region:      "reg",
		Integration: "signup-notify",
		HTTP:        srv.Client(),
		BaseURL:     srv.URL,
		Timeout:     testTimeout,
	}

	if err := c.Notify(context.Background(), "user@example.com", "proj-1", "activated"); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	wantPath := "/v1/projects/proj/locations/reg/integrations/signup-notify:execute"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotBody.TriggerID != triggerID {
		t.Errorf("triggerId = %q, want %q", gotBody.TriggerID, triggerID)
	}
	for key, want := range map[string]string{
		"Signer":  "user@example.com",
		"Project": "proj-1",
		"Status":  "activated",
	} {
		if got := gotBody.InputParameters[key].StringValue; got != want {
			t.Errorf("inputParameters[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestNotifySurfacesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":403}}`, http.StatusForbidden)
	}))
	defer srv.Close()

	c := &Client{
		ProjectID:   "proj",
		Region:      "reg",
		Integration: "signup-notify",
		HTTP:        srv.Client(),
		BaseURL:     srv.URL,
		Timeout:     testTimeout,
	}

	if err := c.Notify(context.Background(), "user@example.com", "proj-1", "activated"); err == nil {
		t.Fatal("Notify: expected error for HTTP 403, got nil")
	}
}
