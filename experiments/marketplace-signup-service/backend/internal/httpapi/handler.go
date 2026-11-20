// Package httpapi exposes the sign-up backend's HTTP surface: health endpoints
// and the JSON sign-up endpoint that the SPA calls through the load balancer's
// /api/* path rule.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/auth"
	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/config"
	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/identity"
	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/workflow"
)

// verificationTimeout bounds the identity, binding, and ownership stages so a
// slow upstream cannot hold the handler open.
const verificationTimeout = 25 * time.Second

// maxRequestBody bounds the JSON body. Two JWTs plus a project ID fit in a few
// kilobytes; anything larger is a client bug or an attempt to exhaust memory.
const maxRequestBody = 64 << 10

// Error codes returned to the SPA. They are the contract the frontend maps to
// user-facing messages, so they are stable strings, not HTTP status text.
const (
	CodeInvalidRequest = "invalid_request" // malformed body or project ID
	CodeUnauthorized   = "unauthorized"    // token invalid or identity not allowed
	CodeAlreadyClaimed = "already_claimed" // namespace administered by another user
	CodeReservedName   = "reserved_name"   // project ID is a reserved namespace
	CodeTransient      = "transient"       // retryable infrastructure failure
	CodeUnavailable    = "unavailable"     // backend misconfiguration; not retryable
)

// projectIDPattern mirrors the GCP project ID syntax, which is also the
// workflow's namespace pattern: 6-30 characters, leading letter, lowercase
// letters/digits/hyphens, no trailing hyphen.
var projectIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)

// The handler's dependencies, expressed as narrow interfaces so the
// verification pipeline is testable without network calls.
type (
	// IDTokenVerifier validates a Google ID token and returns its verified claims.
	IDTokenVerifier interface {
		Verify(ctx context.Context, rawToken string) (auth.Identity, error)
	}
	// Activator runs the activate-project workflow.
	Activator interface {
		Activate(ctx context.Context, namespace, subject string) (workflow.Result, error)
	}
	// Notifier sends the per-activation email. Fire-and-forget: failures are
	// logged and never affect the sign-up response.
	Notifier interface {
		Notify(ctx context.Context, signer, project, status string) error
	}
)

// Deps are the pipeline's collaborators.
type Deps struct {
	Verifier  IDTokenVerifier
	Activator Activator
	// Notifier is optional: nil disables the activation email.
	Notifier Notifier
}

// SignupRequest is the SPA's submission. Identity comes solely from the
// Google ID token; there is no user access token in the interim design.
type SignupRequest struct {
	ProjectID string `json:"project_id"`
	IDToken   string `json:"id_token"`
}

// SignupResponse is the success body.
type SignupResponse struct {
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
}

// Handler serves the API.
type Handler struct {
	cfg           config.Config
	logger        *slog.Logger
	policy        *identity.Policy
	deps          Deps
	notifyTimeout time.Duration
}

// NewHandler builds the HTTP handler and its hosted-domain policy.
func NewHandler(cfg config.Config, logger *slog.Logger, deps Deps, policy *identity.Policy) *Handler {
	return &Handler{
		cfg:           cfg,
		logger:        logger,
		policy:        policy,
		deps:          deps,
		notifyTimeout: 15 * time.Second,
	}
}

// Routes returns the mux for the service.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /readyz", h.health)
	// The load balancer routes /api/* here without rewriting the path.
	mux.HandleFunc("POST /api/v1/signup", h.signup)
	if h.cfg.FrontendDir != "" {
		// Local development only: same-origin SPA without a CORS surface.
		mux.Handle("GET /", http.FileServer(http.Dir(h.cfg.FrontendDir)))
	}
	return securityHeaders(h.cors(mux))
}

// cors allows exactly one origin — the deployed SPA — to call the API. The
// SPA is served from the CDN hostname and the API from the cluster Gateway, so
// they are cross-origin in deployment. Reflecting a single configured origin
// (never a wildcard) keeps other sites from driving sign-ups with a user's
// browser credentials.
func (h *Handler) cors(next http.Handler) http.Handler {
	if h.cfg.AllowedOrigin == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin == h.cfg.AllowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", h.cfg.AllowedOrigin)
			w.Header().Set("Vary", "Origin")

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) signup(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSignupRequest(w, r)
	if err != nil {
		h.logger.Warn("rejected malformed signup request", "error", err)
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "invalid sign-up request")
		return
	}
	if !projectIDPattern.MatchString(req.ProjectID) {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "project_id is not a valid GCP project ID")
		return
	}

	// Stages 1-4 are local or short Google API calls; bound them so a hung
	// upstream cannot pin the handler. Activation gets its own, longer budget.
	verifyCtx, cancel := context.WithTimeout(r.Context(), verificationTimeout)
	defer cancel()

	// 1. Validate the ID token: signature, issuer, expiry, audience.
	verified, err := h.deps.Verifier.Verify(verifyCtx, req.IDToken)
	if err != nil {
		h.reject(w, http.StatusUnauthorized, CodeUnauthorized, req.ProjectID, "id token invalid", err)
		return
	}

	// 2. Enforce the Red Hat hosted-domain policy on the signed claims.
	if err := h.policy.Authorize(verified.HostedDomain, verified.Email); err != nil {
		h.reject(w, http.StatusUnauthorized, CodeUnauthorized, req.ProjectID, "identity not allowed", err)
		return
	}
	email, err := identity.NormalizeEmail(verified.Email)
	if err != nil {
		h.reject(w, http.StatusUnauthorized, CodeUnauthorized, req.ProjectID, "email unusable", err)
		return
	}

	// 3. Interim limitation: project ownership is NOT verified. Google
	// Workspace app-access control blocks the cloud-platform.read-only scope in
	// this org, so there is no user-authorized way to read the project's IAM
	// policy. Only the verified Red Hat identity and the project ID syntax are
	// checked; see design D3 for the accepted risk and the exit options.

	// 4. Activate the namespace through the existing workflow.
	result, err := h.deps.Activator.Activate(r.Context(), req.ProjectID, email)
	if err != nil {
		h.writeActivationError(w, req.ProjectID, err)
		return
	}

	h.logger.Info("signup succeeded",
		"project_id", req.ProjectID,
		"namespace", result.Namespace,
		"subject", email,
		"status", result.Status,
	)

	// Fire-and-forget notification: the activation already succeeded, so a
	// slow or failed email must never affect the response or be retried by
	// the client. The request context dies with the response, so detach it.
	if h.deps.Notifier != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), h.notifyTimeout)
			defer cancel()
			if err := h.deps.Notifier.Notify(ctx, email, req.ProjectID, result.Status); err != nil {
				h.logger.Warn("activation notification failed",
					"project_id", req.ProjectID,
					"error", err,
				)
				return
			}
			h.logger.Info("activation notified",
				"project_id", req.ProjectID,
				"status", result.Status,
			)
		}()
	}

	writeJSON(w, http.StatusOK, SignupResponse{Namespace: result.Namespace, Status: result.Status})
}

func (h *Handler) writeActivationError(w http.ResponseWriter, projectID string, err error) {
	switch {
	case errors.Is(err, workflow.ErrAlreadyClaimed):
		h.reject(w, http.StatusConflict, CodeAlreadyClaimed, projectID, "already claimed", err)
	case errors.Is(err, workflow.ErrReservedName):
		h.reject(w, http.StatusBadRequest, CodeReservedName, projectID, "reserved namespace", err)
	case errors.Is(err, workflow.ErrInvalidInput):
		h.reject(w, http.StatusBadRequest, CodeInvalidRequest, projectID, "workflow rejected arguments", err)
	case errors.Is(err, workflow.ErrMisconfigured):
		// The backend cannot invoke the workflow at all: an operator problem,
		// not something the user can retry away.
		h.logger.Error("signup unavailable: workflow invocation not authorized",
			"project_id", projectID, "error", err)
		writeError(w, http.StatusInternalServerError, CodeUnavailable, messages[CodeUnavailable])
	default:
		h.reject(w, http.StatusServiceUnavailable, CodeTransient, projectID, "activation failed", err)
	}
}

// reject logs the reason (never tokens) and returns the stable error code.
func (h *Handler) reject(w http.ResponseWriter, status int, code, projectID, reason string, err error) {
	h.logger.Warn("signup rejected",
		"project_id", projectID,
		"reason", reason,
		"error", err,
	)
	writeError(w, status, code, messages[code])
}

// messages are deliberately generic: clients learn the category, not the
// details (which could reveal whether a project exists).
var messages = map[string]string{
	CodeInvalidRequest: "invalid sign-up request",
	CodeUnauthorized:   "sign-in could not be verified",
	CodeAlreadyClaimed: "project is already administered by another user",
	CodeReservedName:   "project ID is reserved by the platform",
	CodeTransient:      "temporary failure, please retry",
	CodeUnavailable:    "sign-up is temporarily unavailable",
}

func decodeSignupRequest(w http.ResponseWriter, r *http.Request) (SignupRequest, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	defer r.Body.Close()

	var req SignupRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return SignupRequest{}, err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return SignupRequest{}, errors.New("expected a single JSON object")
	}

	req.ProjectID = strings.TrimSpace(req.ProjectID)
	if req.ProjectID == "" || req.IDToken == "" {
		return SignupRequest{}, errors.New("project_id and id_token are required")
	}
	return req, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message}})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
