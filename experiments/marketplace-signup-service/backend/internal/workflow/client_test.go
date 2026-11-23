package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeWorkflows returns a client backed by a local server that answers the
// create call and then hands out the given execution states in order.
func fakeWorkflows(t *testing.T, states []map[string]any) *Client {
	t.Helper()

	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/projects/p/locations/r/workflows/w/executions":
			var req struct {
				Argument string `json:"argument"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode create body: %v", err)
			}
			var args map[string]string
			if err := json.Unmarshal([]byte(req.Argument), &args); err != nil {
				t.Errorf("argument is not JSON: %q", req.Argument)
			}
			if args["project_namespace"] != "my-project-123" || args["subject"] != "alice@redhat.com" {
				t.Errorf("unexpected argument: %v", args)
			}
			_, _ = w.Write([]byte(`{"name":"projects/p/locations/r/executions/e1","state":"ACTIVE"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/projects/p/locations/r/executions/e1":
			idx := gets
			if idx >= len(states) {
				idx = len(states) - 1
			}
			gets++
			body, _ := json.Marshal(states[idx])
			_, _ = w.Write(body)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return &Client{
		ProjectID:    "p",
		Region:       "r",
		WorkflowName: "w",
		HTTP:         srv.Client(),
		BaseURL:      srv.URL,
		Timeout:      5 * time.Second,
		PollInterval: time.Millisecond,
	}
}

func TestActivateSuccess(t *testing.T) {
	c := fakeWorkflows(t, []map[string]any{
		{"name": "e1", "state": "ACTIVE"},
		{"name": "e1", "state": "SUCCEEDED", "result": `{"status":"activated","namespace":"my-project-123"}`},
	})

	res, err := c.Activate(context.Background(), "my-project-123", "alice@redhat.com")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if res.Status != "activated" || res.Namespace != "my-project-123" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestActivateAlreadyActive(t *testing.T) {
	c := fakeWorkflows(t, []map[string]any{
		{"name": "e1", "state": "SUCCEEDED", "result": `{"status":"already_active","namespace":"my-project-123"}`},
	})
	res, err := c.Activate(context.Background(), "my-project-123", "alice@redhat.com")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if res.Status != "already_active" {
		t.Fatalf("status = %q", res.Status)
	}
}

func TestActivateFailureMapping(t *testing.T) {
	tests := []struct {
		name string
		exec map[string]any
		want error
	}{
		{
			name: "reserved namespace",
			exec: map[string]any{
				"name": "e1", "state": "FAILED",
				"error": map[string]any{"payload": map[string]any{"code": 403.0, "message": "reserved"}},
			},
			want: ErrReservedName,
		},
		{
			name: "subject conflict",
			exec: map[string]any{
				"name": "e1", "state": "FAILED",
				"error": map[string]any{"payload": map[string]any{"code": 409.0, "message": "exists"}},
			},
			want: ErrAlreadyClaimed,
		},
		{
			name: "input validation raise",
			exec: map[string]any{
				"name": "e1", "state": "FAILED",
				"error": map[string]any{"payload": "Invalid project_namespace: expected a valid GCP project ID"},
			},
			want: ErrInvalidInput,
		},
		{
			name: "role unavailable is transient",
			exec: map[string]any{
				"name": "e1", "state": "FAILED",
				"error": map[string]any{"payload": "PlatformRole service-admin is unavailable: timeout"},
			},
			want: ErrTransient,
		},
		{
			name: "gke error is transient",
			exec: map[string]any{
				"name": "e1", "state": "FAILED",
				"error": map[string]any{"payload": "Failed to create service-admin binding: HTTP 500"},
			},
			want: ErrTransient,
		},
		{
			name: "no payload is transient",
			exec: map[string]any{"name": "e1", "state": "FAILED"},
			want: ErrTransient,
		},
		{
			name: "cancelled is transient",
			exec: map[string]any{"name": "e1", "state": "CANCELLED"},
			want: ErrTransient,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := fakeWorkflows(t, []map[string]any{tc.exec})
			_, err := c.Activate(context.Background(), "my-project-123", "alice@redhat.com")
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestActivateCreateFailureIsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"internal"}}`))
	}))
	t.Cleanup(srv.Close)

	c := &Client{
		ProjectID: "p", Region: "r", WorkflowName: "w",
		HTTP: srv.Client(), BaseURL: srv.URL,
		Timeout: time.Second, PollInterval: time.Millisecond,
	}
	if _, err := c.Activate(context.Background(), "my-project-123", "alice@redhat.com"); !errors.Is(err, ErrTransient) {
		t.Fatalf("error = %v, want ErrTransient", err)
	}
}

func TestActivateCreateForbiddenIsMisconfigured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"caller lacks workflows.invoker"}}`))
	}))
	t.Cleanup(srv.Close)

	c := &Client{
		ProjectID: "p", Region: "r", WorkflowName: "w",
		HTTP: srv.Client(), BaseURL: srv.URL,
		Timeout: time.Second, PollInterval: time.Millisecond,
	}
	if _, err := c.Activate(context.Background(), "my-project-123", "alice@redhat.com"); !errors.Is(err, ErrMisconfigured) {
		t.Fatalf("error = %v, want ErrMisconfigured", err)
	}
}

func TestActivateTimesOut(t *testing.T) {
	c := fakeWorkflows(t, []map[string]any{{"name": "e1", "state": "ACTIVE"}})
	c.Timeout = 50 * time.Millisecond
	c.PollInterval = time.Millisecond
	if _, err := c.Activate(context.Background(), "my-project-123", "alice@redhat.com"); !errors.Is(err, ErrTransient) {
		t.Fatalf("error = %v, want ErrTransient", err)
	}
}

func TestClassifyFailureOuterStringPayload(t *testing.T) {
	// Real Executions API wraps the workflow's JSON in an outer JSON string.
	outer409, _ := json.Marshal(map[string]any{"code": 409.0, "message": "exists"})
	wrapped409, _ := json.Marshal(string(outer409))
	if err := classifyFailure(&executionError{Payload: wrapped409, Context: "failed"}); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("wrapped 409: got %v, want ErrAlreadyClaimed", err)
	}
	outer403, _ := json.Marshal(map[string]any{"code": 403.0, "message": "reserved"})
	wrapped403, _ := json.Marshal(string(outer403))
	if err := classifyFailure(&executionError{Payload: wrapped403, Context: "failed"}); !errors.Is(err, ErrReservedName) {
		t.Fatalf("wrapped 403: got %v, want ErrReservedName", err)
	}
	// String payload wrapped as outer string should still classify as input validation
	wrappedStr, _ := json.Marshal("Invalid project_namespace: expected a valid GCP project ID")
	// Need double wrapping: inner string already JSON, outer wraps it
	outerStr, _ := json.Marshal(string(wrappedStr))
	// The actual wire format for string payloads is a single outer string
	// containing the message; test the direct wrapped string case
	directWrapped, _ := json.Marshal("Invalid project_namespace: bad")
	if err := classifyFailure(&executionError{Payload: directWrapped, Context: "failed"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("wrapped input validation: got %v, want ErrInvalidInput", err)
	}
	_ = outerStr
}

func TestActivatePollRetriesTransientThenSucceeds(t *testing.T) {
	// First GET after create returns 502, second returns SUCCEEDED
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			if _, err := w.Write([]byte(`{"name":"projects/p/locations/r/workflows/w/executions/e1","state":"ACTIVE"}`)); err != nil {
				t.Errorf("write create: %v", err)
			}
			return
		}
		if r.Method == http.MethodGet {
			gets++
			if gets == 1 {
				w.WriteHeader(http.StatusBadGateway)
				if _, err := w.Write([]byte(`{"error":"bad gateway"}`)); err != nil {
					t.Errorf("write 502: %v", err)
				}
				return
			}
			body, _ := json.Marshal(map[string]any{
				"name": "e1", "state": "SUCCEEDED",
				"result": `{"status":"activated","namespace":"my-project-123"}`,
			})
			if _, err := w.Write(body); err != nil {
				t.Errorf("write success: %v", err)
			}
			return
		}
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	c := &Client{
		ProjectID: "p", Region: "r", WorkflowName: "w",
		HTTP: srv.Client(), BaseURL: srv.URL,
		Timeout: 5 * time.Second, PollInterval: time.Millisecond,
	}
	res, err := c.Activate(context.Background(), "my-project-123", "alice@redhat.com")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if res.Status != "activated" {
		t.Fatalf("status=%q", res.Status)
	}
	if gets != 2 {
		t.Fatalf("gets=%d, want 2 (retry exercised)", gets)
	}
}

func TestActivatePollDeniedIsTransientNotMisconfigured(t *testing.T) {
	// POST accepted, GET denied — should be transient (outcome unknown), not misconfigured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			if _, err := w.Write([]byte(`{"name":"projects/p/locations/r/workflows/w/executions/e1","state":"ACTIVE"}`)); err != nil {
				t.Errorf("write create: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusForbidden)
		if _, err := w.Write([]byte(`{"error":"forbidden"}`)); err != nil {
			t.Errorf("write 403: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	c := &Client{
		ProjectID: "p", Region: "r", WorkflowName: "w",
		HTTP: srv.Client(), BaseURL: srv.URL,
		Timeout: time.Second, PollInterval: time.Millisecond,
	}
	_, err := c.Activate(context.Background(), "my-project-123", "alice@redhat.com")
	if !errors.Is(err, ErrTransient) {
		t.Fatalf("got %v, want ErrTransient", err)
	}
	if errors.Is(err, ErrMisconfigured) {
		t.Fatalf("must not be ErrMisconfigured (create was accepted, poll was denied)")
	}
}
