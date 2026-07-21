package authn

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/thetechnick/orlop/pkg/apiserver/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// UserInfoKey is the context key for storing authenticated user information.
	UserInfoKey contextKey = "authn.userInfo"
)

// Middleware is an HTTP middleware that enforces authentication.
type Middleware struct {
	authenticator *Authenticator
	logger        logr.Logger
}

// NewMiddleware creates a new authentication middleware.
func NewMiddleware(authenticator *Authenticator, logger logr.Logger) *Middleware {
	return &Middleware{
		authenticator: authenticator,
		logger:        logger,
	}
}

// Handler returns an HTTP middleware function.
func (m *Middleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			token := ExtractBearerToken(authHeader)

			if token == "" {
				// No token provided - set anonymous user
				userInfo := &UserInfo{
					Username: "system:anonymous",
					Groups:   []string{"system:unauthenticated"},
				}
				ctx := context.WithValue(r.Context(), UserInfoKey, userInfo)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Authenticate token
			userInfo, err := m.authenticator.Authenticate(r.Context(), token)
			if err != nil {
				m.logger.V(1).Info("Authentication failed", "error", err.Error())
				m.writeUnauthorized(w, "Invalid authentication credentials")
				return
			}

			m.logger.V(2).Info("Authenticated user", "username", userInfo.Username, "groups", userInfo.Groups)

			// Add user info to context
			ctx := context.WithValue(r.Context(), UserInfoKey, userInfo)

			// Set headers for downstream handlers (e.g., RBAC middleware)
			r.Header.Set("X-Remote-User", userInfo.Username)
			r.Header.Del("X-Remote-Group") // Clear any existing groups
			for _, group := range userInfo.Groups {
				r.Header.Add("X-Remote-Group", group)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeUnauthorized writes a 401 Unauthorized response.
func (m *Middleware) writeUnauthorized(w http.ResponseWriter, message string) {
	status := metav1.Status{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Status",
		},
		Status:  "Failure",
		Message: message,
		Reason:  metav1.StatusReasonUnauthorized,
		Code:    http.StatusUnauthorized,
	}

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(status)
}

// GetUserInfo extracts authenticated user information from the request context.
func GetUserInfo(ctx context.Context) (*UserInfo, bool) {
	userInfo, ok := ctx.Value(UserInfoKey).(*UserInfo)
	return userInfo, ok
}
