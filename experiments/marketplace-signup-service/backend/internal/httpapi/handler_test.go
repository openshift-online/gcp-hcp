package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/auth"
	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/config"
	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/identity"
	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/workflow"
)

const (
	validProject = "my-project-123"
	validBody    = `{"project_id":"my-project-123","id_token":"id.token.sig"}`
)

type fakeVerifier struct {
	id  auth.Identity
	err error
}

func (f *fakeVerifier) Verify(context.Context, string) (auth.Identity, error) { return f.id, f.err }

type fakeActivator struct {
	res   workflow.Result
	err   error
	calls int
}

func (f *fakeActivator) Activate(context.Context, string, string) (workflow.Result, error) {
	f.calls++
	return f.res, f.err
}

// fakeNotifier records Notify calls; err is returned to the handler, which
// must swallow it.
type fakeNotifier struct {
	calls []notifyCall
	err   error
	done  chan struct{}
}

type notifyCall struct {
	signer, project, status string
}

func (f *fakeNotifier) Notify(_ context.Context, signer, project, status string) error {
	f.calls = append(f.calls, notifyCall{signer, project, status})
	if f.done != nil {
		close(f.done)
	}
	return f.err
}

type pipeline struct {
	verifier  *fakeVerifier
	activator *fakeActivator
	notifier  Notifier // *fakeNotifier or *panickingNotifier
	handler   http.Handler
}

func newPipeline(t *testing.T, tweak func(*pipeline)) *pipeline {
	t.Helper()
	p := &pipeline{
		verifier: &fakeVerifier{id: auth.Identity{
			Subject: "sub-1", Email: "alice@redhat.com", EmailVerified: true, HostedDomain: "redhat.com",
		}},
		activator: &fakeActivator{res: workflow.Result{Status: "activated", Namespace: validProject}},
	}
	if tweak != nil {
		tweak(p)
	}
	policy, err := identity.NewPolicy([]string{"redhat.com"})
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	// Assign the notifier only when set: a typed-nil *fakeNotifier would make
	// the Notifier interface non-nil and panic in the handler's goroutine.
	deps := Deps{Verifier: p.verifier, Activator: p.activator}
	if p.notifier != nil {
		deps.Notifier = p.notifier
	}
	p.handler = NewHandler(
		config.Config{OAuthClientID: "test-client"},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		deps,
		policy,
	).Routes()
	return p
}

func post(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func readError(t *testing.T, rec *httptest.ResponseRecorder) apiError {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v (body %q)", err, rec.Body.String())
	}
	return env.Error
}

func expectCode(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, wantStatus, rec.Body.String())
	}
	if got := readError(t, rec).Code; got != wantCode {
		t.Fatalf("code = %q, want %q", got, wantCode)
	}
}

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	newPipeline(t, nil).handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", rec.Code)
	}
}

func TestSignupNotifies(t *testing.T) {
	n := &fakeNotifier{done: make(chan struct{})}
	p := newPipeline(t, func(pl *pipeline) { pl.notifier = n })
	rec := post(t, p.handler, validBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	select {
	case <-n.done:
	case <-time.After(2 * time.Second):
		t.Fatal("notification not attempted")
	}
	got := n.calls[0]
	if got.signer != "alice@redhat.com" || got.project != validProject || got.status != "activated" {
		t.Fatalf("notification args = %+v", got)
	}
}

func TestNotificationFailureDoesNotAffectSignup(t *testing.T) {
	n := &fakeNotifier{err: errors.New("integration down"), done: make(chan struct{})}
	p := newPipeline(t, func(pl *pipeline) { pl.notifier = n })
	rec := post(t, p.handler, validBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 despite notification failure (body %q)", rec.Code, rec.Body.String())
	}
	select {
	case <-n.done:
	case <-time.After(2 * time.Second):
		t.Fatal("notification not attempted")
	}
}

// panickingNotifier exercises the goroutine's recover: a panic in the
// notifier must not crash the process or fail the request.
type panickingNotifier struct{ done chan struct{} }

func (p *panickingNotifier) Notify(context.Context, string, string, string) error {
	close(p.done)
	panic("notifier bug")
}

func TestNotificationPanicIsContained(t *testing.T) {
	n := &panickingNotifier{done: make(chan struct{})}
	p := newPipeline(t, func(pl *pipeline) { pl.notifier = n })
	rec := post(t, p.handler, validBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 despite notifier panic (body %q)", rec.Code, rec.Body.String())
	}
	select {
	case <-n.done:
	case <-time.After(2 * time.Second):
		t.Fatal("notification not attempted")
	}
}

func TestNoNotifierConfigured(t *testing.T) {
	p := newPipeline(t, nil) // notifier is nil
	rec := post(t, p.handler, validBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
}

func TestSignupSuccess(t *testing.T) {
	p := newPipeline(t, nil)
	rec := post(t, p.handler, validBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var resp SignupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Namespace != validProject || resp.Status != "activated" {
		t.Fatalf("response = %+v", resp)
	}
	if p.activator.calls != 1 {
		t.Fatalf("activator calls = %d, want 1", p.activator.calls)
	}
}

func TestSignupAlreadyActiveIsSuccess(t *testing.T) {
	p := newPipeline(t, func(p *pipeline) {
		p.activator.res = workflow.Result{Status: "already_active", Namespace: validProject}
	})
	rec := post(t, p.handler, validBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestSignupRejectsNonRedHatIdentity(t *testing.T) {
	tests := []struct {
		name string
		id   auth.Identity
	}{
		{name: "consumer account", id: auth.Identity{Email: "alice@gmail.com", EmailVerified: true}},
		{name: "foreign workspace", id: auth.Identity{Email: "alice@evil.example", EmailVerified: true, HostedDomain: "evil.example"}},
		{name: "email disagrees with hd", id: auth.Identity{Email: "alice@gmail.com", EmailVerified: true, HostedDomain: "redhat.com"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newPipeline(t, func(p *pipeline) { p.verifier.id = tc.id })
			rec := post(t, p.handler, validBody)
			expectCode(t, rec, http.StatusUnauthorized, CodeUnauthorized)
			if p.activator.calls != 0 {
				t.Fatal("pipeline continued past an identity rejection")
			}
		})
	}
}

func TestSignupRejectsInvalidIDToken(t *testing.T) {
	p := newPipeline(t, func(p *pipeline) { p.verifier.err = auth.ErrInvalidToken })
	rec := post(t, p.handler, validBody)
	expectCode(t, rec, http.StatusUnauthorized, CodeUnauthorized)
	if p.activator.calls != 0 {
		t.Fatal("workflow invoked despite invalid token")
	}
}

func TestSignupReportsMisconfiguration(t *testing.T) {
	p := newPipeline(t, func(p *pipeline) { p.activator.err = workflow.ErrMisconfigured })
	rec := post(t, p.handler, validBody)
	expectCode(t, rec, http.StatusInternalServerError, CodeUnavailable)
}

func TestSignupActivationOutcomes(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "already claimed", err: workflow.ErrAlreadyClaimed, wantStatus: http.StatusConflict, wantCode: CodeAlreadyClaimed},
		{name: "reserved name", err: workflow.ErrReservedName, wantStatus: http.StatusBadRequest, wantCode: CodeReservedName},
		{name: "invalid arguments", err: workflow.ErrInvalidInput, wantStatus: http.StatusBadRequest, wantCode: CodeInvalidRequest},
		{name: "transient", err: workflow.ErrTransient, wantStatus: http.StatusServiceUnavailable, wantCode: CodeTransient},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newPipeline(t, func(p *pipeline) { p.activator.err = tc.err })
			rec := post(t, p.handler, validBody)
			expectCode(t, rec, tc.wantStatus, tc.wantCode)
		})
	}
}

func TestSignupRejectsMalformedRequests(t *testing.T) {
	h := newPipeline(t, nil).handler
	valid := `"project_id":"my-project-123","id_token":"a.b.c"`

	tests := []struct {
		name string
		body string
	}{
		{name: "not json", body: `{`},
		{name: "unknown field", body: `{` + valid + `,"admin":true}`},
		{name: "missing token", body: `{"project_id":"my-project-123"}`},
		{name: "missing project", body: `{"id_token":"a.b.c"}`},
		{name: "trailing object", body: `{` + valid + `}{}`},
		{name: "project starts with digit", body: `{"project_id":"1bad-project","id_token":"a.b.c"}`},
		{name: "project too short", body: `{"project_id":"short","id_token":"a.b.c"}`},
		{name: "project trailing hyphen", body: `{"project_id":"bad-project-","id_token":"a.b.c"}`},
		{name: "project uppercase", body: `{"project_id":"Bad-Project","id_token":"a.b.c"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectCode(t, post(t, h, tc.body), http.StatusBadRequest, CodeInvalidRequest)
		})
	}
}

func TestGetNotAllowedOnSignup(t *testing.T) {
	rec := httptest.NewRecorder()
	newPipeline(t, nil).handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/signup", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestMessagesCoverAllCodes(t *testing.T) {
	for _, code := range []string{CodeInvalidRequest, CodeUnauthorized, CodeAlreadyClaimed, CodeReservedName, CodeTransient, CodeUnavailable} {
		if messages[code] == "" {
			t.Fatalf("no client message for code %q", code)
		}
	}
	if errors.Is(workflow.ErrTransient, nil) {
		t.Fatal("sanity check")
	}
}

func newCORSPipeline(t *testing.T) http.Handler {
	t.Helper()
	policy, err := identity.NewPolicy([]string{"redhat.com"})
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	return NewHandler(
		config.Config{OAuthClientID: "test-client", AllowedOrigin: "https://signup.example.test"},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Deps{
			Verifier:  &fakeVerifier{},
			Activator: &fakeActivator{},
		},
		policy,
	).Routes()
}

func TestCORSPreflightAllowsConfiguredOrigin(t *testing.T) {
	h := newCORSPipeline(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/signup", nil)
	req.Header.Set("Origin", "https://signup.example.test")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://signup.example.test" {
		t.Fatalf("allow-origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Content-Type") {
		t.Fatalf("allow-headers = %q", got)
	}
}

func TestCORSRejectsForeignOrigin(t *testing.T) {
	h := newCORSPipeline(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/signup", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("foreign origin was allowed: %q", got)
	}
}

func TestCORSHeadersOnActualRequest(t *testing.T) {
	h := newCORSPipeline(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://signup.example.test")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://signup.example.test" {
		t.Fatalf("allow-origin = %q", got)
	}
}

func TestSignupRejectedIncludesAuditFields(t *testing.T) {
	var buf bytes.Buffer
	policy, _ := identity.NewPolicy([]string{"redhat.com"})
	h := NewHandler(
		config.Config{OAuthClientID: "test-client"},
		slog.New(slog.NewTextHandler(&buf, nil)),
		Deps{
			Verifier: &fakeVerifier{id: auth.Identity{Email: "alice@redhat.com", EmailVerified: true, HostedDomain: "redhat.com"}},
			Activator: &fakeActivator{err: workflow.ErrAlreadyClaimed},
		},
		policy,
	).Routes()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", strings.NewReader(validBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	logged := buf.String()
	for _, want := range []string{"alice@redhat.com", "my-project-123", "already claimed"} {
		if !strings.Contains(logged, want) {
			t.Fatalf("rejected log missing %q: %q", want, logged)
		}
	}
}

func TestSignupNormalizeBeforeAuthorize(t *testing.T) {
	// NFC normalization: decomposed e + combining accent should normalize and pass.
	decomposed := "alice\u0065\u0301@redhat.com" // e + ́ -> é in NFC
	policy, _ := identity.NewPolicy([]string{"redhat.com"})
	h := NewHandler(
		config.Config{OAuthClientID: "test-client"},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Deps{
			Verifier: &fakeVerifier{id: auth.Identity{Email: decomposed, EmailVerified: true, HostedDomain: "redhat.com"}},
			Activator: &fakeActivator{res: workflow.Result{Status: "activated", Namespace: validProject}},
		},
		policy,
	).Routes()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", strings.NewReader(validBody))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200; decomposed email should be normalized before authorize (body %q)", rec.Code, rec.Body.String())
	}
}
