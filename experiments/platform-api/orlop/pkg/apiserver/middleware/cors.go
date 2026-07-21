package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/cors"
)

// CORSOptions holds CORS configuration.
type CORSOptions struct {
	AllowedOrigins []string
}

// CORS returns a CORS middleware configured with the given options.
func CORS(opts CORSOptions) func(http.Handler) http.Handler {
	allowedOrigins := opts.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}

	// Process origins - if any origin is "*", allow all
	allowAll := false
	for _, origin := range allowedOrigins {
		if strings.TrimSpace(origin) == "*" {
			allowAll = true
			break
		}
	}

	if allowAll {
		allowedOrigins = []string{"*"}
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization", "X-Requested-With"},
		ExposedHeaders:   []string{"Content-Length", "Content-Type"},
		AllowCredentials: !allowAll, // Don't allow credentials with wildcard origin
		MaxAge:           300,
	})
}
