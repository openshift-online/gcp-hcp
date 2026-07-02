// Package manifest provides utilities for Kubernetes manifest validation,
// generation tracking, and discovery.
//
// This package handles:
//   - Manifest validation (apiVersion, kind, name, generation annotation)
//   - Generation annotation extraction and comparison
//   - ManifestWork validation for OCM
//   - Discovery interface for finding resources/manifests
//
// Used by both k8sclient (Kubernetes resources) and maestroclient (ManifestWork).
package manifest

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/constants"
	apperrors "github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	workv1 "open-cluster-management.io/api/work/v1"
)

// Operation represents the type of operation to perform on a resource
type Operation string

const (
	// OperationCreate indicates the resource should be created
	OperationCreate Operation = "create"
	// OperationUpdate indicates the resource should be updated
	OperationUpdate Operation = "update"
	// OperationRecreate indicates the resource should be deleted and recreated
	OperationRecreate Operation = "recreate"
	// OperationSkip indicates no operation is needed (generations match)
	OperationSkip Operation = "skip"
	// OperationDelete indicates the resource should be deleted (lifecycle.delete.when evaluated to true)
	OperationDelete Operation = "delete"
)

// ApplyDecision contains the decision about what operation to perform
// based on comparing generations between an existing resource and a new resource.
type ApplyDecision struct {
	// Operation is the recommended operation based on generation comparison
	Operation Operation
	// Reason explains why this operation was chosen
	Reason string
	// NewGeneration is the generation of the new resource
	NewGeneration int64
	// ExistingGeneration is the generation of the existing resource (0 if not found)
	ExistingGeneration int64
}

// CompareGenerations compares the generation of a new resource against an existing one
// and returns the recommended operation.
//
// Decision logic:
//   - If exists is false: Create (resource doesn't exist)
//   - If generations match: Skip (no changes needed)
//   - If generations differ: Update (apply changes)
//
// This function encapsulates the generation comparison logic used by both
// resource_executor (for k8s resources) and maestroclient (for ManifestWorks).
func CompareGenerations(newGen, existingGen int64, exists bool) ApplyDecision {
	if !exists {
		return ApplyDecision{
			Operation:          OperationCreate,
			Reason:             "resource not found",
			NewGeneration:      newGen,
			ExistingGeneration: 0,
		}
	}

	if existingGen == newGen {
		return ApplyDecision{
			Operation:          OperationSkip,
			Reason:             fmt.Sprintf("generation %d unchanged", existingGen),
			NewGeneration:      newGen,
			ExistingGeneration: existingGen,
		}
	}

	return ApplyDecision{
		Operation:          OperationUpdate,
		Reason:             fmt.Sprintf("generation changed %d->%d", existingGen, newGen),
		NewGeneration:      newGen,
		ExistingGeneration: existingGen,
	}
}

// GetGeneration extracts the generation annotation value from ObjectMeta.
// Returns 0 if the annotation is not found, empty, or cannot be parsed.
//
// This works with any Kubernetes resource that has ObjectMeta, including:
//   - Unstructured objects (via obj.GetAnnotations())
//   - ManifestWork objects (via work.ObjectMeta or work.Annotations)
//   - Any typed Kubernetes resource (via resource.ObjectMeta)
func GetGeneration(meta metav1.ObjectMeta) int64 {
	if meta.Annotations == nil {
		return 0
	}

	genStr, ok := meta.Annotations[constants.AnnotationGeneration]
	if !ok || genStr == "" {
		return 0
	}

	gen, err := strconv.ParseInt(genStr, 10, 64)
	if err != nil {
		return 0
	}

	return gen
}

// GetGenerationFromUnstructured is a convenience wrapper for getting generation
// from unstructured.Unstructured.
// Returns 0 if the resource is nil, has no annotations, or the annotation cannot be parsed.
func GetGenerationFromUnstructured(obj *unstructured.Unstructured) int64 {
	if obj == nil {
		return 0
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return 0
	}
	genStr, ok := annotations[constants.AnnotationGeneration]
	if !ok || genStr == "" {
		return 0
	}
	gen, err := strconv.ParseInt(genStr, 10, 64)
	if err != nil {
		return 0
	}
	return gen
}

// ValidateGeneration validates that the generation annotation exists and is valid
// on ObjectMeta.
// Returns error if:
//   - Annotation is missing
//   - Annotation value is empty
//   - Annotation value cannot be parsed as int64
//   - Annotation value is <= 0 (must be positive)
//
// This is used to validate that templates properly set the generation annotation.
func ValidateGeneration(meta metav1.ObjectMeta) error {
	if meta.Annotations == nil {
		return apperrors.Validation(
			"missing %s annotation", constants.AnnotationGeneration).AsError()
	}

	genStr, ok := meta.Annotations[constants.AnnotationGeneration]
	if !ok {
		return apperrors.Validation(
			"missing %s annotation", constants.AnnotationGeneration).AsError()
	}

	if genStr == "" {
		return apperrors.Validation("%s annotation is empty", constants.AnnotationGeneration).AsError()
	}

	gen, err := strconv.ParseInt(genStr, 10, 64)
	if err != nil {
		return apperrors.Validation(
			"invalid %s annotation value %q: %v", constants.AnnotationGeneration, genStr, err,
		).AsError()
	}

	if gen <= 0 {
		return apperrors.Validation("%s annotation must be > 0, got %d", constants.AnnotationGeneration, gen).AsError()
	}

	return nil
}

// ValidateManifestWorkGeneration validates that the generation annotation exists on both:
// 1. The ManifestWork metadata (required)
// 2. All manifests within the ManifestWork workload (required)
//
// Returns error if any generation annotation is missing or invalid.
// This ensures templates properly set generation annotations throughout the ManifestWork.
func ValidateManifestWorkGeneration(work *workv1.ManifestWork) error {
	if work == nil {
		return apperrors.Validation("work cannot be nil").AsError()
	}

	// Validate ManifestWork-level generation (required)
	if err := ValidateGeneration(work.ObjectMeta); err != nil {
		return apperrors.Validation("ManifestWork %q: %v", work.Name, err).AsError()
	}

	// Validate each manifest has generation annotation (required)
	for i, m := range work.Spec.Workload.Manifests {
		obj := &unstructured.Unstructured{}
		if err := obj.UnmarshalJSON(m.Raw); err != nil {
			return apperrors.Validation("ManifestWork %q manifest[%d]: failed to unmarshal: %v", work.Name, i, err).AsError()
		}

		// Validate generation annotation exists
		if err := ValidateGenerationFromUnstructured(obj); err != nil {
			kind := obj.GetKind()
			name := obj.GetName()
			return apperrors.Validation("ManifestWork %q manifest[%d] %s/%s: %v", work.Name, i, kind, name, err).AsError()
		}
	}

	return nil
}

// ValidateGenerationFromUnstructured validates that the generation annotation exists
// and is valid on an Unstructured object.
// Returns error if:
//   - Object is nil
//   - Annotation is missing
//   - Annotation value is empty
//   - Annotation value cannot be parsed as int64
//   - Annotation value is <= 0 (must be positive)
func ValidateGenerationFromUnstructured(obj *unstructured.Unstructured) error {
	if obj == nil {
		return apperrors.Validation("object cannot be nil").AsError()
	}

	annotations := obj.GetAnnotations()
	if annotations == nil {
		return apperrors.Validation(
			"missing %s annotation", constants.AnnotationGeneration).AsError()
	}

	genStr, ok := annotations[constants.AnnotationGeneration]
	if !ok {
		return apperrors.Validation(
			"missing %s annotation", constants.AnnotationGeneration).AsError()
	}

	if genStr == "" {
		return apperrors.Validation(
			"%s annotation is empty", constants.AnnotationGeneration).AsError()
	}

	gen, err := strconv.ParseInt(genStr, 10, 64)
	if err != nil {
		return apperrors.Validation(
			"invalid %s annotation value %q: %v",
			constants.AnnotationGeneration, genStr, err).AsError()
	}

	if gen <= 0 {
		return apperrors.Validation(
			"%s annotation must be > 0, got %d",
			constants.AnnotationGeneration, gen).AsError()
	}

	return nil
}

// GetLatestGenerationFromList returns the resource with the highest generation annotation
// from a list.
// It sorts by generation annotation (descending) and uses metadata.name as a secondary sort key
// for deterministic behavior when generations are equal.
// Returns nil if the list is nil or empty.
//
// Useful for finding the most recent version of a resource when multiple versions exist.
func GetLatestGenerationFromList(list *unstructured.UnstructuredList) *unstructured.Unstructured {
	if list == nil || len(list.Items) == 0 {
		return nil
	}

	// Copy items to avoid modifying input
	items := make([]unstructured.Unstructured, len(list.Items))
	copy(items, list.Items)

	// Sort by generation annotation (descending) to return the one with the latest generation
	// Secondary sort by metadata.name for consistency when generations are equal
	sort.Slice(items, func(i, j int) bool {
		genI := GetGenerationFromUnstructured(&items[i])
		genJ := GetGenerationFromUnstructured(&items[j])
		if genI != genJ {
			return genI > genJ // Descending order - latest generation first
		}
		// Fall back to metadata.name for deterministic ordering when generations are equal
		return items[i].GetName() < items[j].GetName()
	})

	return &items[0]
}

// =============================================================================
// Discovery Interface and Configuration
// =============================================================================

// Discovery defines the interface for resource/manifest discovery configuration.
// This interface is used by both k8sclient (for K8s resources)
// and maestroclient (for ManifestWork manifests).
type Discovery interface {
	// GetNamespace returns the namespace to search in.
	// Empty string means cluster-scoped or all namespaces.
	GetNamespace() string

	// GetName returns the resource name for single-resource discovery.
	// Empty string means use selector-based discovery.
	GetName() string

	// GetLabelSelector returns the label selector string
	// (e.g., "app=myapp,env=prod").
	// Empty string means no label filtering.
	GetLabelSelector() string

	// IsSingleResource returns true if discovering by name (single resource).
	IsSingleResource() bool
}

// DiscoveryConfig is the default implementation of the Discovery interface.
// Used by both k8sclient and maestroclient for consistent discovery configuration.
type DiscoveryConfig struct {
	// Namespace to search in (empty for cluster-scoped or all namespaces)
	Namespace string

	// ByName specifies the resource name for single-resource discovery.
	// If set, discovery returns a single resource by name.
	ByName string

	// LabelSelector is the label selector string (e.g., "app=myapp,env=prod")
	LabelSelector string
}

// GetNamespace implements Discovery.GetNamespace
func (d *DiscoveryConfig) GetNamespace() string {
	return d.Namespace
}

// GetName implements Discovery.GetName
func (d *DiscoveryConfig) GetName() string {
	return d.ByName
}

// GetLabelSelector implements Discovery.GetLabelSelector
func (d *DiscoveryConfig) GetLabelSelector() string {
	return d.LabelSelector
}

// IsSingleResource implements Discovery.IsSingleResource
func (d *DiscoveryConfig) IsSingleResource() bool {
	return d.ByName != ""
}

// BuildLabelSelector converts a map of labels to a selector string.
// Keys are sorted alphabetically for deterministic output.
// Example: {"env": "prod", "app": "myapp"} -> "app=myapp,env=prod"
func BuildLabelSelector(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(labels))
	for _, k := range keys {
		pairs = append(pairs, k+"="+labels[k])
	}
	return strings.Join(pairs, ",")
}

// MatchesLabels checks if an object's labels match the given label selector.
// Returns true if all selector labels are present in the object's labels.
func MatchesLabels(obj *unstructured.Unstructured, labelSelector string) bool {
	if labelSelector == "" {
		return true
	}

	objLabels := obj.GetLabels()
	if objLabels == nil {
		return false
	}

	// Parse selector string (e.g., "app=myapp,env=prod")
	pairs := strings.Split(labelSelector, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, value := kv[0], kv[1]
		if objLabels[key] != value {
			return false
		}
	}

	return true
}

// DiscoverNestedManifest finds manifests within a parent resource (e.g., ManifestWork)
// that match the discovery criteria. The parent is expected to contain nested manifests at
// spec.workload.manifests (OCM ManifestWork structure).
//
// Parameters:
//   - parent: The parent unstructured resource containing nested manifests
//   - discovery: Discovery configuration (namespace, name, or label selector)
//
// Returns:
//   - List of matching manifests as unstructured objects
//   - The manifest with the highest generation if multiple match
func DiscoverNestedManifest(
	parent *unstructured.Unstructured,
	discovery Discovery,
) (*unstructured.UnstructuredList, error) {
	list := &unstructured.UnstructuredList{}

	if parent == nil || discovery == nil {
		return list, nil
	}

	// Extract spec.workload.manifests from the unstructured parent
	manifests, found, err := unstructured.NestedSlice(
		parent.Object, "spec", "workload", "manifests")
	if err != nil {
		return nil, apperrors.Validation(
			"failed to extract spec.workload.manifests from %q: %v",
			parent.GetName(), err)
	}
	if !found {
		return list, nil
	}

	for i, m := range manifests {
		manifestMap, ok := m.(map[string]interface{})
		if !ok {
			return nil, apperrors.Validation("%q manifest[%d]: unexpected type %T",
				parent.GetName(), i, m)
		}

		obj := &unstructured.Unstructured{Object: manifestMap}

		// Check if manifest matches discovery criteria
		if MatchesDiscoveryCriteria(obj, discovery) {
			list.Items = append(list.Items, *obj)
		}
	}

	return list, nil
}

// EnrichWithResourceStatus finds the matching status.resourceStatus.manifests[] entry
// in the parent resource and merges its statusFeedback and conditions onto the nested object.
// Correlation uses resourceMeta.name + resourceMeta.namespace matching against the nested
// object's metadata.name and metadata.namespace.
// No-op if parent or nested is nil, or if no matching entry is found.
func EnrichWithResourceStatus(parent, nested *unstructured.Unstructured) {
	if parent == nil || nested == nil {
		return
	}

	statusManifests, found, err := unstructured.NestedSlice(
		parent.Object, "status", "resourceStatus", "manifests")
	if err != nil || !found {
		return
	}

	nestedName := nested.GetName()
	nestedNamespace := nested.GetNamespace()

	for _, entry := range statusManifests {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		resourceMeta, ok := entryMap["resourceMeta"].(map[string]interface{})
		if !ok {
			continue
		}

		metaName, ok := resourceMeta["name"].(string)
		if !ok {
			continue
		}
		metaNamespace, ok := resourceMeta["namespace"].(string)
		if !ok {
			continue
		}

		if metaName == nestedName && metaNamespace == nestedNamespace {
			if sf, exists := entryMap["statusFeedback"]; exists {
				nested.Object["statusFeedback"] = sf
			}
			if conds, exists := entryMap["conditions"]; exists {
				nested.Object["conditions"] = conds
			}
			return
		}
	}
}

// MatchesDiscoveryCriteria checks if a resource matches the discovery criteria
// (namespace, name, or labels).
func MatchesDiscoveryCriteria(obj *unstructured.Unstructured, discovery Discovery) bool {
	// Check namespace if specified
	if ns := discovery.GetNamespace(); ns != "" && obj.GetNamespace() != ns {
		return false
	}

	// Check name if single resource discovery
	if discovery.IsSingleResource() {
		return obj.GetName() == discovery.GetName()
	}

	// Check label selector
	return MatchesLabels(obj, discovery.GetLabelSelector())
}
