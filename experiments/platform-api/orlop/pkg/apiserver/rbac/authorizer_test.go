package rbac

import (
	"context"
	"testing"

	rbacv1 "github.com/thetechnick/orlop/apis/private/rbac/v1"
	"github.com/thetechnick/orlop/pkg/apiserver/storage/memory"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// newTestScheme creates a scheme with the rbac v1 types registered.
func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := rbacv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	return scheme
}

// nilAuthorizer creates an Authorizer with nil stores, suitable for testing
// methods that do not access stores (matches, matchesSubject, ruleAllows, rulesAllow).
func nilAuthorizer() *Authorizer {
	return &Authorizer{}
}

func TestMatches(t *testing.T) {
	a := nilAuthorizer()

	tests := []struct {
		name     string
		value    string
		patterns []string
		want     bool
	}{
		{
			name:     "exact match",
			value:    "get",
			patterns: []string{"get", "list"},
			want:     true,
		},
		{
			name:     "wildcard matches anything",
			value:    "delete",
			patterns: []string{"*"},
			want:     true,
		},
		{
			name:     "prefix wildcard matches",
			value:    "testing",
			patterns: []string{"test*"},
			want:     true,
		},
		{
			name:     "prefix wildcard no match",
			value:    "production",
			patterns: []string{"test*"},
			want:     false,
		},
		{
			name:     "no match",
			value:    "delete",
			patterns: []string{"get", "list"},
			want:     false,
		},
		{
			name:     "empty patterns list",
			value:    "get",
			patterns: []string{},
			want:     false,
		},
		{
			name:     "empty value with wildcard",
			value:    "",
			patterns: []string{"*"},
			want:     true,
		},
		{
			name:     "empty value exact match",
			value:    "",
			patterns: []string{""},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.matches(tt.value, tt.patterns)
			if got != tt.want {
				t.Errorf("matches(%q, %v) = %v, want %v", tt.value, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestMatchesSubject(t *testing.T) {
	a := nilAuthorizer()

	tests := []struct {
		name     string
		attrs    Attributes
		subjects []rbacv1.Subject
		want     bool
	}{
		{
			name: "user kind matches",
			attrs: Attributes{
				User:   "alice",
				Groups: []string{"developers"},
			},
			subjects: []rbacv1.Subject{
				{Kind: "User", Name: "alice"},
			},
			want: true,
		},
		{
			name: "group kind matches",
			attrs: Attributes{
				User:   "alice",
				Groups: []string{"developers", "admins"},
			},
			subjects: []rbacv1.Subject{
				{Kind: "Group", Name: "admins"},
			},
			want: true,
		},
		{
			name: "no match wrong user",
			attrs: Attributes{
				User:   "bob",
				Groups: []string{"developers"},
			},
			subjects: []rbacv1.Subject{
				{Kind: "User", Name: "alice"},
			},
			want: false,
		},
		{
			name: "no match wrong group",
			attrs: Attributes{
				User:   "alice",
				Groups: []string{"developers"},
			},
			subjects: []rbacv1.Subject{
				{Kind: "Group", Name: "admins"},
			},
			want: false,
		},
		{
			name: "empty subjects",
			attrs: Attributes{
				User:   "alice",
				Groups: []string{"developers"},
			},
			subjects: []rbacv1.Subject{},
			want:     false,
		},
		{
			name: "multiple subjects first matches",
			attrs: Attributes{
				User:   "alice",
				Groups: []string{"developers"},
			},
			subjects: []rbacv1.Subject{
				{Kind: "User", Name: "alice"},
				{Kind: "User", Name: "bob"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.matchesSubject(tt.attrs, tt.subjects)
			if got != tt.want {
				t.Errorf("matchesSubject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuleAllows(t *testing.T) {
	a := nilAuthorizer()

	tests := []struct {
		name  string
		attrs Attributes
		rule  rbacv1.PolicyRule
		want  bool
	}{
		{
			name: "all fields match",
			attrs: Attributes{
				Verb:     "get",
				APIGroup: "apps",
				Resource: "deployments",
				Name:     "my-app",
			},
			rule: rbacv1.PolicyRule{
				Verbs:     []string{"get", "list"},
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
			},
			want: true,
		},
		{
			name: "verb does not match",
			attrs: Attributes{
				Verb:     "delete",
				APIGroup: "apps",
				Resource: "deployments",
			},
			rule: rbacv1.PolicyRule{
				Verbs:     []string{"get", "list"},
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
			},
			want: false,
		},
		{
			name: "api group does not match",
			attrs: Attributes{
				Verb:     "get",
				APIGroup: "batch",
				Resource: "deployments",
			},
			rule: rbacv1.PolicyRule{
				Verbs:     []string{"get"},
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
			},
			want: false,
		},
		{
			name: "resource does not match",
			attrs: Attributes{
				Verb:     "get",
				APIGroup: "apps",
				Resource: "statefulsets",
			},
			rule: rbacv1.PolicyRule{
				Verbs:     []string{"get"},
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
			},
			want: false,
		},
		{
			name: "resource name restriction allows matching name",
			attrs: Attributes{
				Verb:     "get",
				APIGroup: "apps",
				Resource: "deployments",
				Name:     "my-app",
			},
			rule: rbacv1.PolicyRule{
				Verbs:         []string{"get"},
				APIGroups:     []string{"apps"},
				Resources:     []string{"deployments"},
				ResourceNames: []string{"my-app", "other-app"},
			},
			want: true,
		},
		{
			name: "resource name restriction denies non-matching name",
			attrs: Attributes{
				Verb:     "get",
				APIGroup: "apps",
				Resource: "deployments",
				Name:     "forbidden-app",
			},
			rule: rbacv1.PolicyRule{
				Verbs:         []string{"get"},
				APIGroups:     []string{"apps"},
				Resources:     []string{"deployments"},
				ResourceNames: []string{"my-app"},
			},
			want: false,
		},
		{
			name: "resource name restriction ignored when attrs name is empty",
			attrs: Attributes{
				Verb:     "list",
				APIGroup: "apps",
				Resource: "deployments",
				Name:     "",
			},
			rule: rbacv1.PolicyRule{
				Verbs:         []string{"list"},
				APIGroups:     []string{"apps"},
				Resources:     []string{"deployments"},
				ResourceNames: []string{"my-app"},
			},
			want: true,
		},
		{
			name: "wildcard verb matches",
			attrs: Attributes{
				Verb:     "delete",
				APIGroup: "apps",
				Resource: "deployments",
			},
			rule: rbacv1.PolicyRule{
				Verbs:     []string{"*"},
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
			},
			want: true,
		},
		{
			name: "wildcard api group matches",
			attrs: Attributes{
				Verb:     "get",
				APIGroup: "anything",
				Resource: "deployments",
			},
			rule: rbacv1.PolicyRule{
				Verbs:     []string{"get"},
				APIGroups: []string{"*"},
				Resources: []string{"deployments"},
			},
			want: true,
		},
		{
			name: "wildcard resource matches",
			attrs: Attributes{
				Verb:     "get",
				APIGroup: "apps",
				Resource: "anything",
			},
			rule: rbacv1.PolicyRule{
				Verbs:     []string{"get"},
				APIGroups: []string{"apps"},
				Resources: []string{"*"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.ruleAllows(tt.attrs, tt.rule)
			if got != tt.want {
				t.Errorf("ruleAllows() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRulesAllow(t *testing.T) {
	a := nilAuthorizer()

	t.Run("allows when any rule matches", func(t *testing.T) {
		attrs := Attributes{
			Verb:     "get",
			APIGroup: "apps",
			Resource: "deployments",
		}
		rules := []rbacv1.PolicyRule{
			{
				Verbs:     []string{"list"},
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
			},
			{
				Verbs:     []string{"get"},
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
			},
		}
		if !a.rulesAllow(attrs, rules) {
			t.Error("rulesAllow() = false, want true")
		}
	})

	t.Run("denies when no rule matches", func(t *testing.T) {
		attrs := Attributes{
			Verb:     "delete",
			APIGroup: "apps",
			Resource: "deployments",
		}
		rules := []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
			},
		}
		if a.rulesAllow(attrs, rules) {
			t.Error("rulesAllow() = true, want false")
		}
	})

	t.Run("denies on empty rules", func(t *testing.T) {
		attrs := Attributes{
			Verb:     "get",
			APIGroup: "apps",
			Resource: "deployments",
		}
		if a.rulesAllow(attrs, nil) {
			t.Error("rulesAllow(nil) = true, want false")
		}
	})
}

func TestAuthorize(t *testing.T) {
	scheme := newTestScheme()
	ctx := context.Background()

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

	t.Run("cluster role binding allows cluster-wide access", func(t *testing.T) {
		clusterRoleStore := memory.NewMemoryStore("clusterroles", scheme, clusterRoleGVK)
		clusterRoleBindingStore := memory.NewMemoryStore("clusterrolebindings", scheme, clusterRoleBindingGVK)
		roleStore := memory.NewMemoryStore("roles", scheme, roleGVK)
		roleBindingStore := memory.NewMemoryStore("rolebindings", scheme, roleBindingGVK)

		// Create a ClusterRole
		cr := &rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.GroupVersion.String(),
				Kind:       "ClusterRole",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "admin-role",
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments"},
				},
			},
		}
		if err := clusterRoleStore.Create(ctx, cr); err != nil {
			t.Fatalf("failed to create ClusterRole: %v", err)
		}

		// Create a ClusterRoleBinding
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
				Name:     "admin-role",
			},
		}
		if err := clusterRoleBindingStore.Create(ctx, crb); err != nil {
			t.Fatalf("failed to create ClusterRoleBinding: %v", err)
		}

		authorizer := NewAuthorizer(roleStore, roleBindingStore, clusterRoleStore, clusterRoleBindingStore)

		decision, err := authorizer.Authorize(ctx, Attributes{
			User:     "alice",
			Verb:     "get",
			APIGroup: "apps",
			Resource: "deployments",
		})
		if err != nil {
			t.Fatalf("Authorize() error: %v", err)
		}
		if decision != DecisionAllow {
			t.Errorf("Authorize() = %v, want DecisionAllow", decision)
		}
	})

	t.Run("role binding allows namespace-scoped access", func(t *testing.T) {
		clusterRoleStore := memory.NewMemoryStore("clusterroles", scheme, clusterRoleGVK)
		clusterRoleBindingStore := memory.NewMemoryStore("clusterrolebindings", scheme, clusterRoleBindingGVK)
		roleStore := memory.NewMemoryStore("roles", scheme, roleGVK)
		roleBindingStore := memory.NewMemoryStore("rolebindings", scheme, roleBindingGVK)

		// Create a Role in namespace "production"
		role := &rbacv1.Role{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.GroupVersion.String(),
				Kind:       "Role",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deployer",
				Namespace: "production",
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "create", "update"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments"},
				},
			},
		}
		if err := roleStore.Create(ctx, role); err != nil {
			t.Fatalf("failed to create Role: %v", err)
		}

		// Create a RoleBinding in namespace "production"
		rb := &rbacv1.RoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.GroupVersion.String(),
				Kind:       "RoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deployer-binding",
				Namespace: "production",
			},
			Subjects: []rbacv1.Subject{
				{Kind: "User", Name: "bob"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupVersion.Group,
				Kind:     "Role",
				Name:     "deployer",
			},
		}
		if err := roleBindingStore.Create(ctx, rb); err != nil {
			t.Fatalf("failed to create RoleBinding: %v", err)
		}

		authorizer := NewAuthorizer(roleStore, roleBindingStore, clusterRoleStore, clusterRoleBindingStore)

		decision, err := authorizer.Authorize(ctx, Attributes{
			User:      "bob",
			Verb:      "create",
			Namespace: "production",
			APIGroup:  "apps",
			Resource:  "deployments",
		})
		if err != nil {
			t.Fatalf("Authorize() error: %v", err)
		}
		if decision != DecisionAllow {
			t.Errorf("Authorize() = %v, want DecisionAllow", decision)
		}
	})

	t.Run("denied when no matching bindings", func(t *testing.T) {
		clusterRoleStore := memory.NewMemoryStore("clusterroles", scheme, clusterRoleGVK)
		clusterRoleBindingStore := memory.NewMemoryStore("clusterrolebindings", scheme, clusterRoleBindingGVK)
		roleStore := memory.NewMemoryStore("roles", scheme, roleGVK)
		roleBindingStore := memory.NewMemoryStore("rolebindings", scheme, roleBindingGVK)

		authorizer := NewAuthorizer(roleStore, roleBindingStore, clusterRoleStore, clusterRoleBindingStore)

		decision, err := authorizer.Authorize(ctx, Attributes{
			User:      "eve",
			Verb:      "delete",
			Namespace: "production",
			APIGroup:  "apps",
			Resource:  "deployments",
		})
		if err != nil {
			t.Fatalf("Authorize() error: %v", err)
		}
		if decision != DecisionDeny {
			t.Errorf("Authorize() = %v, want DecisionDeny", decision)
		}
	})

	t.Run("wildcard verb in cluster role", func(t *testing.T) {
		clusterRoleStore := memory.NewMemoryStore("clusterroles", scheme, clusterRoleGVK)
		clusterRoleBindingStore := memory.NewMemoryStore("clusterrolebindings", scheme, clusterRoleBindingGVK)
		roleStore := memory.NewMemoryStore("roles", scheme, roleGVK)
		roleBindingStore := memory.NewMemoryStore("rolebindings", scheme, roleBindingGVK)

		// Create a ClusterRole with wildcard verbs
		cr := &rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.GroupVersion.String(),
				Kind:       "ClusterRole",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "superadmin",
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
				Name: "superadmin-binding",
			},
			Subjects: []rbacv1.Subject{
				{Kind: "Group", Name: "cluster-admins"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     "superadmin",
			},
		}
		if err := clusterRoleBindingStore.Create(ctx, crb); err != nil {
			t.Fatalf("failed to create ClusterRoleBinding: %v", err)
		}

		authorizer := NewAuthorizer(roleStore, roleBindingStore, clusterRoleStore, clusterRoleBindingStore)

		decision, err := authorizer.Authorize(ctx, Attributes{
			User:      "admin-user",
			Groups:    []string{"cluster-admins"},
			Verb:      "delete",
			Namespace: "kube-system",
			APIGroup:  "apps",
			Resource:  "deployments",
			Name:      "critical-app",
		})
		if err != nil {
			t.Fatalf("Authorize() error: %v", err)
		}
		if decision != DecisionAllow {
			t.Errorf("Authorize() = %v, want DecisionAllow", decision)
		}
	})

	t.Run("namespace binding does not grant access to other namespace", func(t *testing.T) {
		clusterRoleStore := memory.NewMemoryStore("clusterroles", scheme, clusterRoleGVK)
		clusterRoleBindingStore := memory.NewMemoryStore("clusterrolebindings", scheme, clusterRoleBindingGVK)
		roleStore := memory.NewMemoryStore("roles", scheme, roleGVK)
		roleBindingStore := memory.NewMemoryStore("rolebindings", scheme, roleBindingGVK)

		// Create a Role in namespace "staging"
		role := &rbacv1.Role{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.GroupVersion.String(),
				Kind:       "Role",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "viewer",
				Namespace: "staging",
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments"},
				},
			},
		}
		if err := roleStore.Create(ctx, role); err != nil {
			t.Fatalf("failed to create Role: %v", err)
		}

		rb := &rbacv1.RoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.GroupVersion.String(),
				Kind:       "RoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "viewer-binding",
				Namespace: "staging",
			},
			Subjects: []rbacv1.Subject{
				{Kind: "User", Name: "bob"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupVersion.Group,
				Kind:     "Role",
				Name:     "viewer",
			},
		}
		if err := roleBindingStore.Create(ctx, rb); err != nil {
			t.Fatalf("failed to create RoleBinding: %v", err)
		}

		authorizer := NewAuthorizer(roleStore, roleBindingStore, clusterRoleStore, clusterRoleBindingStore)

		// Bob should be denied in "production" even though he has access in "staging"
		decision, err := authorizer.Authorize(ctx, Attributes{
			User:      "bob",
			Verb:      "get",
			Namespace: "production",
			APIGroup:  "apps",
			Resource:  "deployments",
		})
		if err != nil {
			t.Fatalf("Authorize() error: %v", err)
		}
		if decision != DecisionDeny {
			t.Errorf("Authorize() = %v, want DecisionDeny", decision)
		}
	})
}
