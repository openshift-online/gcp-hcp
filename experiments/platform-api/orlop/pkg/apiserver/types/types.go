package types

import (
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
)

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
}
