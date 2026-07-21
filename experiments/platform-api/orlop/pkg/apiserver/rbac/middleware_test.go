package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
	rbacv1 "github.com/thetechnick/orlop/apis/private/rbac/v1"
	"github.com/thetechnick/orlop/pkg/apiserver/constants"
	"github.com/thetechnick/orlop/pkg/apiserver/storage/memory"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// nilMiddleware creates a Middleware with nil authorizer and a discard logger,
// suitable for testing extraction methods that do not use the authorizer.
func nilMiddleware() *Middleware {
	return &Middleware{
		logger: logr.Discard(),
	}
}

func TestExtractUser(t *testing.T) {
	m := nilMiddleware()

	tests := []struct {
		name     string
		header   string
		wantUser string
	}{
		{
			name:     "header present",
			header:   "alice",
			wantUser: "alice",
		},
		{
			name:     "header missing returns system anonymous",
			header:   "",
			wantUser: "system:anonymous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				r.Header.Set("X-Remote-User", tt.header)
			}

			got := m.extractUser(r)
			if got != tt.wantUser {
				t.Errorf("extractUser() = %q, want %q", got, tt.wantUser)
			}
		})
	}
}

func TestExtractGroups(t *testing.T) {
	m := nilMiddleware()

	tests := []struct {
		name       string
		groups     []string
		wantGroups []string
	}{
		{
			name:       "multiple group headers",
			groups:     []string{"developers", "admins"},
			wantGroups: []string{"developers", "admins"},
		},
		{
			name:       "single group header",
			groups:     []string{"operators"},
			wantGroups: []string{"operators"},
		},
		{
			name:       "no headers returns system unauthenticated",
			groups:     nil,
			wantGroups: []string{"system:unauthenticated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			for _, g := range tt.groups {
				r.Header.Add("X-Remote-Group", g)
			}

			got := m.extractGroups(r)
			if len(got) != len(tt.wantGroups) {
				t.Fatalf("extractGroups() returned %d groups, want %d", len(got), len(tt.wantGroups))
			}
			for i, g := range got {
				if g != tt.wantGroups[i] {
					t.Errorf("extractGroups()[%d] = %q, want %q", i, g, tt.wantGroups[i])
				}
			}
		})
	}
}

func TestExtractVerb(t *testing.T) {
	m := nilMiddleware()

	tests := []struct {
		name     string
		method   string
		path     string
		query    string
		chiName  string // value for chi URL param "name"
		wantVerb string
	}{
		{
			name:     "GET without name param returns list",
			method:   http.MethodGet,
			path:     "/apis/apps/v1/namespaces/default/deployments",
			wantVerb: "list",
		},
		{
			name:     "GET with name param returns get",
			method:   http.MethodGet,
			path:     "/apis/apps/v1/namespaces/default/deployments/my-app",
			chiName:  "my-app",
			wantVerb: "get",
		},
		{
			name:     "POST returns create",
			method:   http.MethodPost,
			path:     "/apis/apps/v1/namespaces/default/deployments",
			wantVerb: "create",
		},
		{
			name:     "PUT returns update",
			method:   http.MethodPut,
			path:     "/apis/apps/v1/namespaces/default/deployments/my-app",
			wantVerb: "update",
		},
		{
			name:     "PATCH returns patch",
			method:   http.MethodPatch,
			path:     "/apis/apps/v1/namespaces/default/deployments/my-app",
			wantVerb: "patch",
		},
		{
			name:     "DELETE returns delete",
			method:   http.MethodDelete,
			path:     "/apis/apps/v1/namespaces/default/deployments/my-app",
			wantVerb: "delete",
		},
		{
			name:     "watch query param returns watch",
			method:   http.MethodGet,
			path:     "/apis/apps/v1/namespaces/default/deployments",
			query:    "watch=true",
			wantVerb: "watch",
		},
		{
			name:     "watch takes priority over method",
			method:   http.MethodGet,
			path:     "/apis/apps/v1/namespaces/default/deployments/my-app",
			query:    "watch=true",
			chiName:  "my-app",
			wantVerb: "watch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.path
			if tt.query != "" {
				url += "?" + tt.query
			}
			r := httptest.NewRequest(tt.method, url, nil)

			// Set up chi route context with URL params
			rctx := chi.NewRouteContext()
			if tt.chiName != "" {
				rctx.URLParams.Add(constants.URLParamName, tt.chiName)
			}
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			got := m.extractVerb(r)
			if got != tt.wantVerb {
				t.Errorf("extractVerb() = %q, want %q", got, tt.wantVerb)
			}
		})
	}
}

func TestExtractAttributes(t *testing.T) {
	m := nilMiddleware()

	t.Run("parses namespaced API path", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/apis/apps/v1/namespaces/production/deployments/my-app", nil)
		r.Header.Set("X-Remote-User", "alice")
		r.Header.Add("X-Remote-Group", "developers")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add(constants.URLParamNamespace, "production")
		rctx.URLParams.Add(constants.URLParamName, "my-app")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		attrs := m.extractAttributes(r)

		if attrs.User != "alice" {
			t.Errorf("User = %q, want %q", attrs.User, "alice")
		}
		if len(attrs.Groups) != 1 || attrs.Groups[0] != "developers" {
			t.Errorf("Groups = %v, want [developers]", attrs.Groups)
		}
		if attrs.Verb != "get" {
			t.Errorf("Verb = %q, want %q", attrs.Verb, "get")
		}
		if attrs.Namespace != "production" {
			t.Errorf("Namespace = %q, want %q", attrs.Namespace, "production")
		}
		if attrs.APIGroup != "apps" {
			t.Errorf("APIGroup = %q, want %q", attrs.APIGroup, "apps")
		}
		if attrs.Resource != "deployments" {
			t.Errorf("Resource = %q, want %q", attrs.Resource, "deployments")
		}
		if attrs.Name != "my-app" {
			t.Errorf("Name = %q, want %q", attrs.Name, "my-app")
		}
	})

	t.Run("parses cluster-scoped API path", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/apis/rbac.orlop.thetechnick.ninja/v1/clusterroles/admin", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add(constants.URLParamName, "admin")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		attrs := m.extractAttributes(r)

		if attrs.APIGroup != "rbac.orlop.thetechnick.ninja" {
			t.Errorf("APIGroup = %q, want %q", attrs.APIGroup, "rbac.orlop.thetechnick.ninja")
		}
		if attrs.Resource != "clusterroles" {
			t.Errorf("Resource = %q, want %q", attrs.Resource, "clusterroles")
		}
		if attrs.Name != "admin" {
			t.Errorf("Name = %q, want %q", attrs.Name, "admin")
		}
		if attrs.Namespace != "" {
			t.Errorf("Namespace = %q, want empty", attrs.Namespace)
		}
	})

	t.Run("defaults for anonymous user", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/apis/apps/v1/namespaces/default/deployments", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add(constants.URLParamNamespace, "default")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		attrs := m.extractAttributes(r)

		if attrs.User != "system:anonymous" {
			t.Errorf("User = %q, want %q", attrs.User, "system:anonymous")
		}
		if len(attrs.Groups) != 1 || attrs.Groups[0] != "system:unauthenticated" {
			t.Errorf("Groups = %v, want [system:unauthenticated]", attrs.Groups)
		}
		if attrs.Verb != "create" {
			t.Errorf("Verb = %q, want %q", attrs.Verb, "create")
		}
	})
}

func TestHandler(t *testing.T) {
	scheme := newTestScheme()

	clusterRoleGVK := schema.GroupVersionKind{
		Group:   rbacv1.GroupVersion.Group,
		Version: rbacv1.GroupVersion.Version,
		Kind:    "ClusterRole",
	}
	clusterRoleBindingGVK := schema.GroupVersionKind{
		Group:   rbacv1.GroupVersion.Group,
		Version: rbacv1.GroupVersion.Version,
		Kind:    "ClusterRoleBinding",
	}
	roleGVK := schema.GroupVersionKind{
		Group:   rbacv1.GroupVersion.Group,
		Version: rbacv1.GroupVersion.Version,
		Kind:    "Role",
	}
	roleBindingGVK := schema.GroupVersionKind{
		Group:   rbacv1.GroupVersion.Group,
		Version: rbacv1.GroupVersion.Version,
		Kind:    "RoleBinding",
	}

	t.Run("authorized request passes through", func(t *testing.T) {
		ctx := context.Background()
		clusterRoleStore := memory.NewMemoryStore("clusterroles", scheme, clusterRoleGVK)
		clusterRoleBindingStore := memory.NewMemoryStore("clusterrolebindings", scheme, clusterRoleBindingGVK)
		roleStore := memory.NewMemoryStore("roles", scheme, roleGVK)
		roleBindingStore := memory.NewMemoryStore("rolebindings", scheme, roleBindingGVK)

		// Create a ClusterRole that allows everything
		cr := &rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.GroupVersion.String(),
				Kind:       "ClusterRole",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "admin",
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"*"},
					APIGroups: []string{"*"},
					Resources: []string{"*"},
				},
			},
		}
		if err := clusterRoleStore.Create(ctx, cr); err != nil {
			t.Fatalf("failed to create ClusterRole: %v", err)
		}

		crb := &rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.GroupVersion.String(),
				Kind:       "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "admin-binding",
			},
			Subjects: []rbacv1.Subject{
				{Kind: "User", Name: "alice"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     "admin",
			},
		}
		if err := clusterRoleBindingStore.Create(ctx, crb); err != nil {
			t.Fatalf("failed to create ClusterRoleBinding: %v", err)
		}

		authorizer := NewAuthorizer(roleStore, roleBindingStore, clusterRoleStore, clusterRoleBindingStore)
		mw := NewMiddleware(authorizer, logr.Discard())

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		// Build a chi router so that URL params are properly set
		router := chi.NewRouter()
		router.Use(mw.Handler())
		router.Get("/apis/{group}/{version}/namespaces/{namespace}/{resource}", next)

		req := httptest.NewRequest(http.MethodGet, "/apis/apps/v1/namespaces/default/deployments", nil)
		req.Header.Set("X-Remote-User", "alice")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if !nextCalled {
			t.Error("next handler was not called for authorized request")
		}
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("denied request returns 403", func(t *testing.T) {
		// Empty stores mean no roles or bindings exist, so all requests are denied
		clusterRoleStore := memory.NewMemoryStore("clusterroles", scheme, clusterRoleGVK)
		clusterRoleBindingStore := memory.NewMemoryStore("clusterrolebindings", scheme, clusterRoleBindingGVK)
		roleStore := memory.NewMemoryStore("roles", scheme, roleGVK)
		roleBindingStore := memory.NewMemoryStore("rolebindings", scheme, roleBindingGVK)

		authorizer := NewAuthorizer(roleStore, roleBindingStore, clusterRoleStore, clusterRoleBindingStore)
		mw := NewMiddleware(authorizer, logr.Discard())

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		router := chi.NewRouter()
		router.Use(mw.Handler())
		router.Get("/apis/{group}/{version}/namespaces/{namespace}/{resource}", next)

		req := httptest.NewRequest(http.MethodGet, "/apis/apps/v1/namespaces/default/deployments", nil)
		req.Header.Set("X-Remote-User", "unauthorized-user")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if nextCalled {
			t.Error("next handler should not be called for denied request")
		}
		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})
}
