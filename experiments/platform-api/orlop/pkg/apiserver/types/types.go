package types

import (
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
)

// ParentResourceInfo declares that this resource is a child of another resource.
// When set, nested routes are registered under the parent resource.
type ParentResourceInfo struct {
	// Plural is the plural name of the parent resource (e.g., "clusters").
	Plural string
	// IDField is the dot-separated JSON field path in the child resource
	// that holds the parent's ID (e.g., "spec.clusterID").
	IDField string
}

// ResourceInfo describes a single API resource type.
type ResourceInfo struct {
	// GVK is the GroupVersionKind for this resource
	GVK runtimeschema.GroupVersionKind
	// Plural is the plural name for the resource (e.g., "objects")
	Plural string
	// Singular is the singular name for the resource (e.g., "object")
	Singular string
	// Namespaced indicates whether the resource is namespace-scoped (true) or cluster-scoped (false).
	Namespaced bool
	// SchemaYAML is the OpenAPI v3 schema in YAML format
	SchemaYAML string
	// ParentResource is optional. When set, nested routes are also registered
	// under the parent resource (e.g., /clusters/{clusterID}/nodepools).
	ParentResource *ParentResourceInfo
}
