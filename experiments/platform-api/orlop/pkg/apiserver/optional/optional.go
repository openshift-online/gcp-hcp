package optional

import (
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/thetechnick/orlop/pkg/apiserver"
	"github.com/thetechnick/orlop/pkg/apiserver/authn"
	"github.com/thetechnick/orlop/pkg/apiserver/rbac"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SetupAuthentication creates and returns an authentication middleware.
// It registers ServiceAccount and Secret resource types into the registry and creates an authenticator.
func SetupAuthentication(registry *apiserver.ResourceRegistry, logger logr.Logger) (func(http.Handler) http.Handler, error) {
	authResources := []apiserver.ResourceInfo{
		{
			GVK:    schema.GroupVersionKind{Group: "rbac.orlop.thetechnick.ninja", Version: "v1", Kind: "ServiceAccount"},
			Plural: "serviceaccounts",
		},
		{
			GVK:    schema.GroupVersionKind{Group: "rbac.orlop.thetechnick.ninja", Version: "v1", Kind: "Secret"},
			Plural: "secrets",
		},
	}

	for _, res := range authResources {
		if err := registry.Register(res); err != nil {
			return nil, fmt.Errorf("failed to register authentication resource %s: %w", res.Plural, err)
		}
	}

	serviceAccountStore := registry.GetStore("serviceaccounts")
	secretStore := registry.GetStore("secrets")

	authenticator := authn.NewAuthenticator(serviceAccountStore, secretStore)
	middleware := authn.NewMiddleware(authenticator, logger)
	return middleware.Handler(), nil
}

// SetupRBAC creates and returns an RBAC middleware.
// It registers RBAC resource types into the registry and creates an authorizer.
func SetupRBAC(registry *apiserver.ResourceRegistry, logger logr.Logger) (func(http.Handler) http.Handler, error) {
	rbacResources := []apiserver.ResourceInfo{
		{
			GVK:    schema.GroupVersionKind{Group: "rbac.orlop.thetechnick.ninja", Version: "v1", Kind: "Role"},
			Plural: "roles",
		},
		{
			GVK:    schema.GroupVersionKind{Group: "rbac.orlop.thetechnick.ninja", Version: "v1", Kind: "RoleBinding"},
			Plural: "rolebindings",
		},
		{
			GVK:    schema.GroupVersionKind{Group: "rbac.orlop.thetechnick.ninja", Version: "v1", Kind: "ClusterRole"},
			Plural: "clusterroles",
		},
		{
			GVK:    schema.GroupVersionKind{Group: "rbac.orlop.thetechnick.ninja", Version: "v1", Kind: "ClusterRoleBinding"},
			Plural: "clusterrolebindings",
		},
	}

	for _, res := range rbacResources {
		if err := registry.Register(res); err != nil {
			return nil, fmt.Errorf("failed to register RBAC resource %s: %w", res.Plural, err)
		}
	}

	roleStore := registry.GetStore("roles")
	roleBindingStore := registry.GetStore("rolebindings")
	clusterRoleStore := registry.GetStore("clusterroles")
	clusterRoleBindingStore := registry.GetStore("clusterrolebindings")

	authorizer := rbac.NewAuthorizer(
		roleStore,
		roleBindingStore,
		clusterRoleStore,
		clusterRoleBindingStore,
	)

	middleware := rbac.NewMiddleware(authorizer, logger)
	return middleware.Handler(), nil
}
