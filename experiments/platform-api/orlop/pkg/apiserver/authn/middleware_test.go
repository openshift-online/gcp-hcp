package authn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	rbacv1 "github.com/thetechnick/orlop/apis/private/rbac/v1"
	"github.com/thetechnick/orlop/pkg/apiserver/storage/memory"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetUserInfo(t *testing.T) {
	t.Run("returns false when no user info in context", func(t *testing.T) {
		ctx := context.Background()

		_, ok := GetUserInfo(ctx)
		if ok {
			t.Error("expected ok to be false for context without user info")
		}
	})

	t.Run("returns true and correct info when set", func(t *testing.T) {
		userInfo := &UserInfo{
			Username: "system:serviceaccount:default:my-sa",
			UID:      "uid-123",
			Groups:   []string{"system:serviceaccounts", "system:serviceaccounts:default"},
		}
		ctx := context.WithValue(context.Background(), UserInfoKey, userInfo)

		got, ok := GetUserInfo(ctx)
		if !ok {
			t.Fatal("expected ok to be true")
		}
		if got.Username != userInfo.Username {
			t.Errorf("username = %q, want %q", got.Username, userInfo.Username)
		}
		if got.UID != userInfo.UID {
			t.Errorf("uid = %q, want %q", got.UID, userInfo.UID)
		}
		if len(got.Groups) != len(userInfo.Groups) {
			t.Fatalf("groups length = %d, want %d", len(got.Groups), len(userInfo.Groups))
		}
		for i, g := range got.Groups {
			if g != userInfo.Groups[i] {
				t.Errorf("groups[%d] = %q, want %q", i, g, userInfo.Groups[i])
			}
		}
	})
}

// setupMiddleware creates a Middleware backed by in-memory stores containing
// a ServiceAccount and its token Secret. Returns the middleware and token string.
func setupMiddleware(t *testing.T) (*Middleware, string) {
	t.Helper()

	ctx := context.Background()
	scheme := newTestScheme()

	saStore := memory.NewMemoryStore(
		"serviceaccounts",
		scheme,
		schema.GroupVersionKind{
			Group:   rbacv1.GroupVersion.Group,
			Version: rbacv1.GroupVersion.Version,
			Kind:    "ServiceAccount",
		},
	)
	secretStore := memory.NewMemoryStore(
		"secrets",
		scheme,
		schema.GroupVersionKind{
			Group:   rbacv1.GroupVersion.Group,
			Version: rbacv1.GroupVersion.Version,
			Kind:    "Secret",
		},
	)

	sa := &rbacv1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.GroupVersion.String(),
			Kind:       "ServiceAccount",
		},
	}
	sa.SetName("test-sa")
	sa.SetNamespace("test-ns")
	sa.SetUID(types.UID("sa-uid-001"))

	if err := saStore.Create(ctx, sa); err != nil {
		t.Fatalf("failed to create ServiceAccount: %v", err)
	}

	secret, err := CreateServiceAccountToken(sa)
	if err != nil {
		t.Fatalf("failed to create token secret: %v", err)
	}
	secret.TypeMeta = metav1.TypeMeta{
		APIVersion: rbacv1.GroupVersion.String(),
		Kind:       "Secret",
	}

	token := string(secret.Data[TokenKey])

	if err := secretStore.Create(ctx, secret); err != nil {
		t.Fatalf("failed to create Secret: %v", err)
	}

	auth := NewAuthenticator(saStore, secretStore)
	mw := NewMiddleware(auth, logr.Discard())
	return mw, token
}

func TestMiddleware_Handler(t *testing.T) {
	t.Run("no authorization header sets anonymous user", func(t *testing.T) {
		mw, _ := setupMiddleware(t)

		var capturedUserInfo *UserInfo
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := GetUserInfo(r.Context())
			if !ok {
				t.Error("expected user info in context")
				return
			}
			capturedUserInfo = info
			w.WriteHeader(http.StatusOK)
		})

		handler := mw.Handler()(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
		}
		if capturedUserInfo == nil {
			t.Fatal("next handler was not called")
		}
		if capturedUserInfo.Username != "system:anonymous" {
			t.Errorf("username = %q, want %q", capturedUserInfo.Username, "system:anonymous")
		}
		expectedGroups := []string{"system:unauthenticated"}
		if len(capturedUserInfo.Groups) != len(expectedGroups) {
			t.Fatalf("groups length = %d, want %d", len(capturedUserInfo.Groups), len(expectedGroups))
		}
		if capturedUserInfo.Groups[0] != expectedGroups[0] {
			t.Errorf("groups[0] = %q, want %q", capturedUserInfo.Groups[0], expectedGroups[0])
		}
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		mw, _ := setupMiddleware(t)

		nextCalled := false
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		handler := mw.Handler()(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods", nil)
		req.Header.Set("Authorization", "Bearer invalid-token-value")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
		if nextCalled {
			t.Error("next handler should not have been called for invalid token")
		}

		// Verify the response body is a proper Status object.
		var status metav1.Status
		if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if status.Status != "Failure" {
			t.Errorf("status.Status = %q, want %q", status.Status, "Failure")
		}
		if status.Code != http.StatusUnauthorized {
			t.Errorf("status.Code = %d, want %d", status.Code, http.StatusUnauthorized)
		}
	})

	t.Run("valid token sets authenticated user in context and headers", func(t *testing.T) {
		mw, token := setupMiddleware(t)

		var capturedUserInfo *UserInfo
		var capturedRemoteUser string
		var capturedRemoteGroups []string
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := GetUserInfo(r.Context())
			if !ok {
				t.Error("expected user info in context")
				return
			}
			capturedUserInfo = info
			capturedRemoteUser = r.Header.Get("X-Remote-User")
			capturedRemoteGroups = r.Header.Values("X-Remote-Group")
			w.WriteHeader(http.StatusOK)
		})

		handler := mw.Handler()(nextHandler)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
		}
		if capturedUserInfo == nil {
			t.Fatal("next handler was not called")
		}

		expectedUsername := "system:serviceaccount:test-ns:test-sa"
		if capturedUserInfo.Username != expectedUsername {
			t.Errorf("context username = %q, want %q", capturedUserInfo.Username, expectedUsername)
		}
		if capturedUserInfo.UID != "sa-uid-001" {
			t.Errorf("context uid = %q, want %q", capturedUserInfo.UID, "sa-uid-001")
		}

		// Verify headers set for downstream handlers.
		if capturedRemoteUser != expectedUsername {
			t.Errorf("X-Remote-User header = %q, want %q", capturedRemoteUser, expectedUsername)
		}

		expectedGroups := []string{
			"system:serviceaccounts",
			"system:serviceaccounts:test-ns",
		}
		if len(capturedRemoteGroups) != len(expectedGroups) {
			t.Fatalf("X-Remote-Group header count = %d, want %d", len(capturedRemoteGroups), len(expectedGroups))
		}
		for i, g := range capturedRemoteGroups {
			if g != expectedGroups[i] {
				t.Errorf("X-Remote-Group[%d] = %q, want %q", i, g, expectedGroups[i])
			}
		}
	})
}
