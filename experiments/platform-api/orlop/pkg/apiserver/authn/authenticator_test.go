package authn

import (
	"context"
	"testing"

	rbacv1 "github.com/thetechnick/orlop/apis/private/rbac/v1"
	"github.com/thetechnick/orlop/pkg/apiserver/storage/memory"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		want       string
	}{
		{
			name:       "empty string",
			authHeader: "",
			want:       "",
		},
		{
			name:       "valid bearer token",
			authHeader: "Bearer mytoken",
			want:       "mytoken",
		},
		{
			name:       "basic auth scheme",
			authHeader: "Basic xxx",
			want:       "",
		},
		{
			name:       "bearer with empty token",
			authHeader: "Bearer ",
			want:       "",
		},
		{
			name:       "bearer without space",
			authHeader: "Bearertoken",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractBearerToken(tt.authHeader)
			if got != tt.want {
				t.Errorf("ExtractBearerToken(%q) = %q, want %q", tt.authHeader, got, tt.want)
			}
		})
	}
}

func TestGenerateToken(t *testing.T) {
	t.Run("returns non-empty string", func(t *testing.T) {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() returned error: %v", err)
		}
		if token == "" {
			t.Error("GenerateToken() returned empty string")
		}
	})

	t.Run("returns different tokens on successive calls", func(t *testing.T) {
		token1, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() first call returned error: %v", err)
		}

		token2, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() second call returned error: %v", err)
		}

		if token1 == token2 {
			t.Error("GenerateToken() returned identical tokens on two calls")
		}
	})
}

func TestCreateServiceAccountToken(t *testing.T) {
	t.Run("creates secret with correct name", func(t *testing.T) {
		sa := &rbacv1.ServiceAccount{}
		sa.SetName("my-sa")
		sa.SetNamespace("default")
		sa.SetUID(types.UID("test-uid-123"))

		secret, err := CreateServiceAccountToken(sa)
		if err != nil {
			t.Fatalf("CreateServiceAccountToken() returned error: %v", err)
		}

		expectedName := "my-sa-token"
		if secret.GetName() != expectedName {
			t.Errorf("secret name = %q, want %q", secret.GetName(), expectedName)
		}
		if secret.GetNamespace() != "default" {
			t.Errorf("secret namespace = %q, want %q", secret.GetNamespace(), "default")
		}
	})

	t.Run("secret has correct annotations", func(t *testing.T) {
		sa := &rbacv1.ServiceAccount{}
		sa.SetName("my-sa")
		sa.SetNamespace("default")
		sa.SetUID(types.UID("test-uid-123"))

		secret, err := CreateServiceAccountToken(sa)
		if err != nil {
			t.Fatalf("CreateServiceAccountToken() returned error: %v", err)
		}

		annotations := secret.GetAnnotations()
		if annotations[ServiceAccountNameKey] != "my-sa" {
			t.Errorf("annotation %s = %q, want %q", ServiceAccountNameKey, annotations[ServiceAccountNameKey], "my-sa")
		}
		if annotations[ServiceAccountUIDKey] != "test-uid-123" {
			t.Errorf("annotation %s = %q, want %q", ServiceAccountUIDKey, annotations[ServiceAccountUIDKey], "test-uid-123")
		}
	})

	t.Run("secret has correct type", func(t *testing.T) {
		sa := &rbacv1.ServiceAccount{}
		sa.SetName("my-sa")
		sa.SetNamespace("default")

		secret, err := CreateServiceAccountToken(sa)
		if err != nil {
			t.Fatalf("CreateServiceAccountToken() returned error: %v", err)
		}

		if secret.Type != ServiceAccountTokenType {
			t.Errorf("secret type = %q, want %q", secret.Type, ServiceAccountTokenType)
		}
	})

	t.Run("secret has token in data", func(t *testing.T) {
		sa := &rbacv1.ServiceAccount{}
		sa.SetName("my-sa")
		sa.SetNamespace("default")

		secret, err := CreateServiceAccountToken(sa)
		if err != nil {
			t.Fatalf("CreateServiceAccountToken() returned error: %v", err)
		}

		tokenData, ok := secret.Data[TokenKey]
		if !ok {
			t.Fatal("secret.Data missing token key")
		}
		if len(tokenData) == 0 {
			t.Error("secret.Data token is empty")
		}
	})
}

// newTestScheme creates a runtime.Scheme with the rbac/v1 types registered.
func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := rbacv1.AddToScheme(scheme); err != nil {
		panic("failed to add rbac/v1 types to scheme: " + err.Error())
	}
	return scheme
}

// newServiceAccountStore creates a MemoryStore for ServiceAccount objects.
func newServiceAccountStore(scheme *runtime.Scheme) *memory.MemoryStore {
	return memory.NewMemoryStore(
		"serviceaccounts",
		scheme,
		schema.GroupVersionKind{
			Group:   rbacv1.GroupVersion.Group,
			Version: rbacv1.GroupVersion.Version,
			Kind:    "ServiceAccount",
		},
	)
}

// newSecretStore creates a MemoryStore for Secret objects.
func newSecretStore(scheme *runtime.Scheme) *memory.MemoryStore {
	return memory.NewMemoryStore(
		"secrets",
		scheme,
		schema.GroupVersionKind{
			Group:   rbacv1.GroupVersion.Group,
			Version: rbacv1.GroupVersion.Version,
			Kind:    "Secret",
		},
	)
}

func TestAuthenticator_Authenticate(t *testing.T) {
	// setupAuthenticator creates a fully wired Authenticator with a ServiceAccount
	// and matching Secret in the stores. Returns the authenticator and the token string.
	setupAuthenticator := func(t *testing.T) (*Authenticator, string) {
		t.Helper()

		ctx := context.Background()
		scheme := newTestScheme()
		saStore := newServiceAccountStore(scheme)
		secretStore := newSecretStore(scheme)

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
		return auth, token
	}

	t.Run("empty token returns error", func(t *testing.T) {
		auth, _ := setupAuthenticator(t)

		_, err := auth.Authenticate(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty token, got nil")
		}
	})

	t.Run("invalid token returns error", func(t *testing.T) {
		auth, _ := setupAuthenticator(t)

		_, err := auth.Authenticate(context.Background(), "not-a-valid-token")
		if err == nil {
			t.Fatal("expected error for invalid token, got nil")
		}
	})

	t.Run("valid token returns correct UserInfo", func(t *testing.T) {
		auth, token := setupAuthenticator(t)

		userInfo, err := auth.Authenticate(context.Background(), token)
		if err != nil {
			t.Fatalf("Authenticate() returned error: %v", err)
		}

		expectedUsername := "system:serviceaccount:test-ns:test-sa"
		if userInfo.Username != expectedUsername {
			t.Errorf("username = %q, want %q", userInfo.Username, expectedUsername)
		}

		if userInfo.UID != "sa-uid-001" {
			t.Errorf("uid = %q, want %q", userInfo.UID, "sa-uid-001")
		}

		expectedGroups := []string{
			"system:serviceaccounts",
			"system:serviceaccounts:test-ns",
		}
		if len(userInfo.Groups) != len(expectedGroups) {
			t.Fatalf("groups length = %d, want %d", len(userInfo.Groups), len(expectedGroups))
		}
		for i, g := range userInfo.Groups {
			if g != expectedGroups[i] {
				t.Errorf("groups[%d] = %q, want %q", i, g, expectedGroups[i])
			}
		}
	})
}
