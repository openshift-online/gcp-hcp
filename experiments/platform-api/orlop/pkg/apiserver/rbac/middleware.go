package rbac

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
	"github.com/thetechnick/orlop/pkg/apiserver/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Middleware is an HTTP middleware that enforces RBAC authorization.
type Middleware struct {
	authorizer *Authorizer
	logger     logr.Logger
}

// NewMiddleware creates a new RBAC middleware.
func NewMiddleware(authorizer *Authorizer, logger logr.Logger) *Middleware {
	return &Middleware{
		authorizer: authorizer,
		logger:     logger,
	}
}

// Handler returns an HTTP middleware function.
func (m *Middleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract authorization attributes from request
			attrs := m.extractAttributes(r)

			// Check authorization
			decision, err := m.authorizer.Authorize(r.Context(), attrs)
			if err != nil {
				m.logger.Error(err, "Authorization check failed", "user", attrs.User, "verb", attrs.Verb, "resource", attrs.Resource)
				m.writeForbidden(w, "Authorization error")
				return
			}

			if decision != DecisionAllow {
				m.logger.V(1).Info("Access denied",
					"user", attrs.User,
					"verb", attrs.Verb,
					"namespace", attrs.Namespace,
					"apiGroup", attrs.APIGroup,
					"resource", attrs.Resource,
					"name", attrs.Name,
				)
				m.writeForbidden(w, "Access denied")
				return
			}

			// Authorized, continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// extractAttributes extracts authorization attributes from the HTTP request.
func (m *Middleware) extractAttributes(r *http.Request) Attributes {
	attrs := Attributes{
		User:   m.extractUser(r),
		Groups: m.extractGroups(r),
		Verb:   m.extractVerb(r),
	}

	// Extract namespace from URL path
	if ns := chi.URLParam(r, constants.URLParamNamespace); ns != "" {
		attrs.Namespace = ns
	}

	// Extract API group and resource from URL path
	// URL format: /apis/{group}/{version}/namespaces/{namespace}/{resource}/{name}
	// or: /apis/{group}/{version}/{resource}/{name} for cluster-scoped
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) >= 3 && pathParts[0] == "apis" {
		// Extract group from path
		attrs.APIGroup = pathParts[1]

		// Extract resource
		if len(pathParts) >= 5 {
			if pathParts[3] == "namespaces" && len(pathParts) >= 6 {
				// Namespaced resource
				attrs.Resource = pathParts[5]
				if len(pathParts) >= 7 {
					attrs.Name = pathParts[6]
				}
			} else {
				// Cluster-scoped resource
				attrs.Resource = pathParts[3]
				if len(pathParts) >= 5 {
					attrs.Name = pathParts[4]
				}
			}
		}
	}

	return attrs
}

// extractUser extracts the user identity from the request.
// Looks for X-Remote-User header or falls back to "anonymous".
func (m *Middleware) extractUser(r *http.Request) string {
	if user := r.Header.Get("X-Remote-User"); user != "" {
		return user
	}
	return "system:anonymous"
}

// extractGroups extracts user groups from the request.
// Looks for X-Remote-Group headers.
func (m *Middleware) extractGroups(r *http.Request) []string {
	groups := r.Header.Values("X-Remote-Group")
	if len(groups) == 0 {
		groups = []string{"system:unauthenticated"}
	}
	return groups
}

// extractVerb maps HTTP method to Kubernetes verb.
func (m *Middleware) extractVerb(r *http.Request) string {
	// Check for watch parameter
	if r.URL.Query().Get(constants.QueryParamWatch) == "true" {
		return "watch"
	}

	// Map HTTP method to verb
	switch r.Method {
	case http.MethodGet:
		// Distinguish between get and list based on path
		if strings.Contains(r.URL.Path, "/") && chi.URLParam(r, constants.URLParamName) != "" {
			return "get"
		}
		return "list"
	case http.MethodPost:
		return "create"
	case http.MethodPut:
		return "update"
	case http.MethodPatch:
		return "patch"
	case http.MethodDelete:
		return "delete"
	default:
		return strings.ToLower(r.Method)
	}
}

// writeForbidden writes a 403 Forbidden response.
func (m *Middleware) writeForbidden(w http.ResponseWriter, message string) {
	status := metav1.Status{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Status",
		},
		Status:  "Failure",
		Message: message,
		Reason:  metav1.StatusReasonForbidden,
		Code:    http.StatusForbidden,
	}

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(status)
}
