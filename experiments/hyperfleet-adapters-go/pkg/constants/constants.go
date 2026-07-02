package constants

// HyperFleet Kubernetes Resource Annotations and Labels
// These constants define standard annotations and labels used across HyperFleet resources
// for tracking, management, and identification purposes.

const (
	// AnnotationGeneration is the annotation key for tracking resource generation.
	// This is used to track changes and ensure resources are updated with the correct generation.
	// Format: "hyperfleet.io/generation"
	// Example value: "5" (integer as string)
	AnnotationGeneration = "hyperfleet.io/generation"

	// LabelGeneration is the label key for tracking resource generation.
	// Used for label-based filtering and selection of resources by generation.
	// Format: "hyperfleet.io/generation"
	// Example value: "5" (integer as string)
	LabelGeneration = "hyperfleet.io/generation"

	// AnnotationClusterID is the annotation key for cluster identification.
	// Links resources to their target cluster.
	// Format: "hyperfleet.io/cluster-id"
	AnnotationClusterID = "hyperfleet.io/cluster-id"

	// LabelClusterID is the label key for cluster identification.
	// Used for label-based filtering and selection of resources by cluster.
	// Format: "hyperfleet.io/cluster-id"
	LabelClusterID = "hyperfleet.io/cluster-id"

	// AnnotationAdapter is the annotation key for adapter identification.
	// Identifies which adapter created or manages the resource.
	// Format: "hyperfleet.io/adapter"
	AnnotationAdapter = "hyperfleet.io/adapter"

	// LabelAdapter is the label key for adapter identification.
	// Used for label-based filtering and selection of resources by adapter.
	// Format: "hyperfleet.io/adapter"
	LabelAdapter = "hyperfleet.io/adapter"

	// AnnotationManagedBy identifies the entity managing the resource.
	// Format: "hyperfleet.io/managed-by"
	// Example value: "hyperfleet-adapter"
	AnnotationManagedBy = "hyperfleet.io/managed-by"

	// LabelManagedBy is the label key for managed-by identification.
	// Format: "hyperfleet.io/managed-by"
	LabelManagedBy = "hyperfleet.io/managed-by"

	// AnnotationCreatedBy identifies the entity that created the resource.
	// Format: "hyperfleet.io/created-by"
	// Example value: "hyperfleet-adapter"
	AnnotationCreatedBy = "hyperfleet.io/created-by"
)

// OCM ManifestWork GVK constants
const (
	// ManifestWorkGroup is the API group for OCM ManifestWork resources.
	ManifestWorkGroup = "work.open-cluster-management.io"

	// ManifestWorkVersion is the API version for OCM ManifestWork resources.
	ManifestWorkVersion = "v1"

	// ManifestWorkKind is the Kind for OCM ManifestWork resources.
	ManifestWorkKind = "ManifestWork"
)
