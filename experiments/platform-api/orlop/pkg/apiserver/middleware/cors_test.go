package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("default allows all origins", func(t *testing.T) {
		handler := CORS(CORSOptions{})(dummyHandler)

		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "GET")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		acao := rr.Header().Get("Access-Control-Allow-Origin")
		if acao != "*" {
			t.Errorf("expected Access-Control-Allow-Origin *, got %q", acao)
		}
	})

	t.Run("specific origins are passed through", func(t *testing.T) {
		handler := CORS(CORSOptions{
			AllowedOrigins: []string{"https://example.com"},
		})(dummyHandler)

		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "GET")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		acao := rr.Header().Get("Access-Control-Allow-Origin")
		if acao != "https://example.com" {
			t.Errorf("expected Access-Control-Allow-Origin https://example.com, got %q", acao)
		}
	})

	t.Run("wildcard in list collapses to wildcard", func(t *testing.T) {
		handler := CORS(CORSOptions{
			AllowedOrigins: []string{"https://example.com", "*"},
		})(dummyHandler)

		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://other.com")
		req.Header.Set("Access-Control-Request-Method", "GET")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		acao := rr.Header().Get("Access-Control-Allow-Origin")
		if acao != "*" {
			t.Errorf("expected wildcard origin, got %q", acao)
		}
	})

	t.Run("allowed methods include PATCH", func(t *testing.T) {
		handler := CORS(CORSOptions{})(dummyHandler)

		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "PATCH")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code == http.StatusMethodNotAllowed {
			t.Error("PATCH should be an allowed method")
		}
	})

	t.Run("non-preflight request passes through", func(t *testing.T) {
		handler := CORS(CORSOptions{})(dummyHandler)

		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}
