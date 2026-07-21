package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	rbacv1 "github.com/thetechnick/orlop/apis/private/rbac/v1"
	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Authorizer checks if a user has permission to perform an action.
type Authorizer struct {
	roleStore               storage.ResourceStore
	roleBindingStore        storage.ResourceStore
	clusterRoleStore        storage.ResourceStore
	clusterRoleBindingStore storage.ResourceStore
}

// NewAuthorizer creates a new RBAC authorizer.
func NewAuthorizer(
	roleStore storage.ResourceStore,
	roleBindingStore storage.ResourceStore,
	clusterRoleStore storage.ResourceStore,
	clusterRoleBindingStore storage.ResourceStore,
) *Authorizer {
	return &Authorizer{
		roleStore:               roleStore,
		roleBindingStore:        roleBindingStore,
		clusterRoleStore:        clusterRoleStore,
		clusterRoleBindingStore: clusterRoleBindingStore,
	}
}

// Attributes describes information about a request being evaluated.
type Attributes struct {
	User      string // User identity
	Groups    []string
	Verb      string // get, list, create, update, delete, watch
	Namespace string
	APIGroup  string
	Resource  string
	Name      string // Resource name (empty for list/create)
}

// Decision represents an authorization decision.
type Decision int

const (
	DecisionDeny Decision = iota
	DecisionAllow
	DecisionNoOpinion
)

// Authorize checks if the given attributes are allowed.
func (a *Authorizer) Authorize(ctx context.Context, attrs Attributes) (Decision, error) {
	// Check cluster-wide bindings first
	decision, err := a.checkClusterRoleBindings(ctx, attrs)
	if err != nil {
		return DecisionDeny, err
	}
	if decision == DecisionAllow {
		return DecisionAllow, nil
	}

	// Check namespace-scoped bindings
	if attrs.Namespace != "" {
		decision, err := a.checkRoleBindings(ctx, attrs)
		if err != nil {
			return DecisionDeny, err
		}
		if decision == DecisionAllow {
			return DecisionAllow, nil
		}
	}

	return DecisionDeny, nil
}

// checkClusterRoleBindings checks cluster-level role bindings.
func (a *Authorizer) checkClusterRoleBindings(ctx context.Context, attrs Attributes) (Decision, error) {
	// List all ClusterRoleBindings
	bindingList, err := a.clusterRoleBindingStore.List(ctx, storage.ListOptions{})
	if err != nil {
		return DecisionNoOpinion, fmt.Errorf("failed to list cluster role bindings: %w", err)
	}

	bindings, err := extractClusterRoleBindings(bindingList)
	if err != nil {
		return DecisionNoOpinion, err
	}

	for _, binding := range bindings {
		// Check if user matches any subject
		if !a.matchesSubject(attrs, binding.Subjects) {
			continue
		}

		// Get the referenced ClusterRole
		if binding.RoleRef.Kind != "ClusterRole" {
			continue
		}

		role, err := a.clusterRoleStore.Get(ctx, "", binding.RoleRef.Name)
		if err != nil {
			continue // Skip if role not found
		}

		// Convert to ClusterRole
		clusterRole, err := convertToClusterRole(role)
		if err != nil {
			continue
		}

		// Check if any rule allows the request
		if a.rulesAllow(attrs, clusterRole.Rules) {
			return DecisionAllow, nil
		}
	}

	return DecisionNoOpinion, nil
}

// checkRoleBindings checks namespace-level role bindings.
func (a *Authorizer) checkRoleBindings(ctx context.Context, attrs Attributes) (Decision, error) {
	// List RoleBindings in the namespace
	bindingList, err := a.roleBindingStore.List(ctx, storage.ListOptions{
		Namespace: attrs.Namespace,
	})
	if err != nil {
		return DecisionNoOpinion, fmt.Errorf("failed to list role bindings: %w", err)
	}

	bindings, err := extractRoleBindings(bindingList)
	if err != nil {
		return DecisionNoOpinion, err
	}

	for _, binding := range bindings {
		// Check if user matches any subject
		if !a.matchesSubject(attrs, binding.Subjects) {
			continue
		}

		// Get the referenced role (Role or ClusterRole)
		var rules []rbacv1.PolicyRule
		if binding.RoleRef.Kind == "Role" {
			role, err := a.roleStore.Get(ctx, attrs.Namespace, binding.RoleRef.Name)
			if err != nil {
				continue
			}
			r, err := convertToRole(role)
			if err != nil {
				continue
			}
			rules = r.Rules
		} else if binding.RoleRef.Kind == "ClusterRole" {
			role, err := a.clusterRoleStore.Get(ctx, "", binding.RoleRef.Name)
			if err != nil {
				continue
			}
			cr, err := convertToClusterRole(role)
			if err != nil {
				continue
			}
			rules = cr.Rules
		}

		// Check if any rule allows the request
		if a.rulesAllow(attrs, rules) {
			return DecisionAllow, nil
		}
	}

	return DecisionNoOpinion, nil
}

// matchesSubject checks if the user/groups match any subject.
func (a *Authorizer) matchesSubject(attrs Attributes, subjects []rbacv1.Subject) bool {
	for _, subject := range subjects {
		switch subject.Kind {
		case "User":
			if subject.Name == attrs.User {
				return true
			}
		case "Group":
			for _, group := range attrs.Groups {
				if subject.Name == group {
					return true
				}
			}
		}
	}
	return false
}

// rulesAllow checks if any rule allows the request.
func (a *Authorizer) rulesAllow(attrs Attributes, rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		if a.ruleAllows(attrs, rule) {
			return true
		}
	}
	return false
}

// ruleAllows checks if a single rule allows the request.
func (a *Authorizer) ruleAllows(attrs Attributes, rule rbacv1.PolicyRule) bool {
	// Check verb
	if !a.matches(attrs.Verb, rule.Verbs) {
		return false
	}

	// Check API group
	if !a.matches(attrs.APIGroup, rule.APIGroups) {
		return false
	}

	// Check resource
	if !a.matches(attrs.Resource, rule.Resources) {
		return false
	}

	// Check resource name if specified
	if len(rule.ResourceNames) > 0 && attrs.Name != "" {
		if !a.matches(attrs.Name, rule.ResourceNames) {
			return false
		}
	}

	return true
}

// matches checks if a value matches any pattern in the list.
// Supports wildcards "*".
func (a *Authorizer) matches(value string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "*" {
			return true
		}
		if pattern == value {
			return true
		}
		// Support prefix matching with wildcards
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(value, prefix) {
				return true
			}
		}
	}
	return false
}

// extractRoleBindings extracts RoleBinding objects from a list.
func extractRoleBindings(list client.ObjectList) ([]*rbacv1.RoleBinding, error) {
	// Convert via JSON marshaling
	data, err := json.Marshal(list)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal list: %w", err)
	}

	var bindingList rbacv1.RoleBindingList
	if err := json.Unmarshal(data, &bindingList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to RoleBindingList: %w", err)
	}

	result := make([]*rbacv1.RoleBinding, len(bindingList.Items))
	for i := range bindingList.Items {
		result[i] = &bindingList.Items[i]
	}
	return result, nil
}

// extractClusterRoleBindings extracts ClusterRoleBinding objects from a list.
func extractClusterRoleBindings(list client.ObjectList) ([]*rbacv1.ClusterRoleBinding, error) {
	// Convert via JSON marshaling
	data, err := json.Marshal(list)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal list: %w", err)
	}

	var bindingList rbacv1.ClusterRoleBindingList
	if err := json.Unmarshal(data, &bindingList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to ClusterRoleBindingList: %w", err)
	}

	result := make([]*rbacv1.ClusterRoleBinding, len(bindingList.Items))
	for i := range bindingList.Items {
		result[i] = &bindingList.Items[i]
	}
	return result, nil
}

// convertToRole converts a client.Object to a Role.
func convertToRole(obj client.Object) (*rbacv1.Role, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object: %w", err)
	}

	var role rbacv1.Role
	if err := json.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to Role: %w", err)
	}

	return &role, nil
}

// convertToClusterRole converts a client.Object to a ClusterRole.
func convertToClusterRole(obj client.Object) (*rbacv1.ClusterRole, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object: %w", err)
	}

	var role rbacv1.ClusterRole
	if err := json.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to ClusterRole: %w", err)
	}

	return &role, nil
}
