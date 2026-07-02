package maestroclient

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/manifest"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/constants"
	apperrors "github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/errors"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubetypes "k8s.io/apimachinery/pkg/types"
	workv1 "open-cluster-management.io/api/work/v1"
)

// CreateManifestWork creates a new ManifestWork for a target cluster (consumer)
//
// The ManifestWork object should be pre-constructed from a template with:
//   - hyperfleet.io/generation annotation on ManifestWork metadata
//   - hyperfleet.io/generation annotation on each manifest within the workload
//
// This method validates that generation annotations are present and sets the namespace.
//
// Parameters:
//   - ctx: Context for the operation
//   - consumerName: The target cluster name (Maestro consumer) - will be set as namespace
//   - work: Pre-constructed ManifestWork object (from template with generation annotations)
//
// Returns the created ManifestWork or an error
func (c *Client) CreateManifestWork(
	ctx context.Context,
	consumerName string,
	work *workv1.ManifestWork,
) (*workv1.ManifestWork, error) {
	if work == nil {
		return nil, apperrors.MaestroError("work for manifestwork cannot be nil")
	}

	// Validate that generation annotations are present (required on ManifestWork and all manifests)
	if err := manifest.ValidateManifestWorkGeneration(work); err != nil {
		return nil, apperrors.MaestroError("invalid ManifestWork: %v", err)
	}

	// Enrich context with common fields
	ctx = logger.WithMaestroConsumer(ctx, consumerName)
	ctx = logger.WithLogField(ctx, "manifestwork", work.Name)
	ctx = logger.WithObservedGeneration(ctx, manifest.GetGeneration(work.ObjectMeta))

	// Set namespace to consumer name (required by Maestro)
	work.Namespace = consumerName

	// Create via the work client
	created, err := c.workClient.ManifestWorks(consumerName).Create(ctx, work, metav1.CreateOptions{})
	if err != nil {
		if isConsumerNotFoundError(err) {
			return nil, apperrors.NotFound("consumer %q is not registered in Maestro", consumerName)
		}
		return nil, apperrors.MaestroError("failed to create ManifestWork %s/%s: %v",
			consumerName, work.Name, err)
	}

	return created, nil
}

// GetManifestWork retrieves a ManifestWork by name from a target cluster
func (c *Client) GetManifestWork(
	ctx context.Context,
	consumerName string,
	workName string,
) (*workv1.ManifestWork, error) {
	ctx = logger.WithMaestroConsumer(ctx, consumerName)
	ctx = logger.WithLogField(ctx, "manifestwork", workName)

	work, err := c.workClient.ManifestWorks(consumerName).Get(ctx, workName, metav1.GetOptions{})
	if err != nil {
		// Return not found error without wrapping for callers to check
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, apperrors.MaestroError("failed to get ManifestWork %s/%s: %v",
			consumerName, workName, err)
	}

	return work, nil
}

// PatchManifestWork patches an existing ManifestWork using JSON merge patch
func (c *Client) PatchManifestWork(
	ctx context.Context,
	consumerName string,
	workName string,
	patchData []byte,
) (*workv1.ManifestWork, error) {
	ctx = logger.WithMaestroConsumer(ctx, consumerName)
	ctx = logger.WithLogField(ctx, "manifestwork", workName)

	patched, err := c.workClient.ManifestWorks(consumerName).Patch(
		ctx,
		workName,
		kubetypes.MergePatchType,
		patchData,
		metav1.PatchOptions{},
	)
	if err != nil {
		return nil, apperrors.MaestroError("failed to patch ManifestWork %s/%s: %v",
			consumerName, workName, err)
	}

	return patched, nil
}

// DeleteManifestWork deletes a ManifestWork from a target cluster
func (c *Client) DeleteManifestWork(
	ctx context.Context,
	consumerName string,
	workName string,
) error {
	ctx = logger.WithMaestroConsumer(ctx, consumerName)
	ctx = logger.WithLogField(ctx, "manifestwork", workName)

	err := c.workClient.ManifestWorks(consumerName).Delete(ctx, workName, metav1.DeleteOptions{})
	if err != nil {
		// Ignore not found errors (already deleted)
		if apierrors.IsNotFound(err) {
			return nil
		}
		return apperrors.MaestroError("failed to delete ManifestWork %s/%s: %v",
			consumerName, workName, err)
	}

	return nil
}

// ListManifestWorks lists all ManifestWorks for a target cluster
func (c *Client) ListManifestWorks(
	ctx context.Context,
	consumerName string,
	labelSelector string,
) (*workv1.ManifestWorkList, error) {
	ctx = logger.WithMaestroConsumer(ctx, consumerName)

	opts := metav1.ListOptions{}
	if labelSelector != "" {
		opts.LabelSelector = labelSelector
	}

	list, err := c.workClient.ManifestWorks(consumerName).List(ctx, opts)
	if err != nil {
		return nil, apperrors.MaestroError("failed to list ManifestWorks for consumer %s: %v",
			consumerName, err)
	}

	return list, nil
}

// ApplyManifestWork creates or updates a ManifestWork (upsert operation)
//
// If the ManifestWork doesn't exist, it creates it.
// If it exists and the generation differs, it updates the ManifestWork.
// If it exists and the generation matches, it skips the update (idempotent).
//
// The ManifestWork object should be pre-constructed from a template with:
//   - hyperfleet.io/generation annotation on ManifestWork metadata
//   - hyperfleet.io/generation annotation on each manifest within the workload
//
// Parameters:
//   - ctx: Context for the operation
//   - consumerName: The target cluster name (will be set as namespace)
//   - work: Pre-constructed ManifestWork object (from template with generation annotations)
//
// Returns the created or updated ManifestWork or an error
func (c *Client) ApplyManifestWork(
	ctx context.Context,
	consumerName string,
	manifestWork *workv1.ManifestWork,
) (*ApplyManifestWorkResult, error) {
	if manifestWork == nil {
		return nil, apperrors.MaestroError("work cannot be nil")
	}

	// Validate that generation annotations are present (required on ManifestWork and all manifests)
	if err := manifest.ValidateManifestWorkGeneration(manifestWork); err != nil {
		return nil, apperrors.MaestroError("invalid ManifestWork: %v", err)
	}

	// Get generation from the work (set by template)
	newGeneration := manifest.GetGeneration(manifestWork.ObjectMeta)

	// Enrich context with common fields
	ctx = logger.WithMaestroConsumer(ctx, consumerName)
	ctx = logger.WithLogField(ctx, "manifestwork", manifestWork.Name)
	ctx = logger.WithObservedGeneration(ctx, newGeneration)

	// Check if ManifestWork exists
	existing, err := c.GetManifestWork(ctx, consumerName, manifestWork.Name)
	exists := err == nil
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Get existing generation (0 if not found)
	var existingGeneration int64
	if exists {
		existingGeneration = manifest.GetGeneration(existing.ObjectMeta)
	}

	// Compare generations to determine operation
	decision := manifest.CompareGenerations(newGeneration, existingGeneration, exists)

	c.log.WithFields(map[string]interface{}{
		"operation": decision.Operation,
		"reason":    decision.Reason,
	}).Debug(ctx, "Apply operation determined")

	// Execute operation based on comparison result
	switch decision.Operation {
	case manifest.OperationCreate:
		work, createErr := c.CreateManifestWork(ctx, consumerName, manifestWork)
		if createErr != nil {
			return nil, createErr
		}
		return &ApplyManifestWorkResult{Work: work, Operation: decision.Operation, Reason: decision.Reason}, nil
	case manifest.OperationSkip:
		return &ApplyManifestWorkResult{Work: existing, Operation: decision.Operation, Reason: decision.Reason}, nil
	case manifest.OperationUpdate:
		patchData, patchErr := createManifestWorkPatch(manifestWork)
		if patchErr != nil {
			return nil, apperrors.MaestroError("failed to create patch: %v", patchErr)
		}
		work, patchErr := c.PatchManifestWork(ctx, consumerName, manifestWork.Name, patchData)
		if patchErr != nil {
			return nil, patchErr
		}
		return &ApplyManifestWorkResult{Work: work, Operation: decision.Operation, Reason: decision.Reason}, nil
	default:
		return nil, apperrors.MaestroError("unexpected operation: %s", decision.Operation)
	}
}

// createManifestWorkPatch creates a JSON merge patch for updating a ManifestWork
func createManifestWorkPatch(work *workv1.ManifestWork) ([]byte, error) {
	// Create patch with metadata (labels, annotations) and spec
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels":      work.Labels,
			"annotations": work.Annotations,
		},
		"spec": work.Spec,
	}
	return json.Marshal(patch)
}

// DiscoverManifest finds manifests within a ManifestWork that match the discovery criteria.
// This is the maestroclient equivalent of k8sclient.DiscoverResources.
//
// Parameters:
//   - ctx: Context for the operation
//   - consumerName: The target cluster name (Maestro consumer)
//   - workName: Name of the ManifestWork to search within
//   - discovery: Discovery configuration (namespace, name, or label selector)
//
// Returns:
//   - List of matching manifests as unstructured objects
//   - Error if ManifestWork not found or discovery fails
//
// Example:
//
//	discovery := &manifest.DiscoveryConfig{
//	    Namespace:     "default",
//	    LabelSelector: "app=myapp",
//	}
//	list, err := client.DiscoverManifest(ctx, "cluster-1", "my-work", discovery)
func (c *Client) DiscoverManifest(
	ctx context.Context,
	consumerName string,
	workName string,
	discovery manifest.Discovery,
) (*unstructured.UnstructuredList, error) {
	ctx = logger.WithMaestroConsumer(ctx, consumerName)
	ctx = logger.WithLogField(ctx, "manifestwork", workName)

	// Get the ManifestWork
	work, err := c.GetManifestWork(ctx, consumerName, workName)
	if err != nil {
		return nil, err
	}

	// Convert typed ManifestWork to unstructured for shared discovery logic
	workUnstructured, err := workToUnstructured(work)
	if err != nil {
		return nil, apperrors.MaestroError("failed to convert ManifestWork %s/%s to unstructured: %v",
			consumerName, workName, err)
	}

	// Use shared discovery logic from manifest package
	list, err := manifest.DiscoverNestedManifest(workUnstructured, discovery)
	if err != nil {
		return nil, apperrors.MaestroError("failed to discover manifests in %s/%s: %v",
			consumerName, workName, err)
	}

	return list, nil
}

// DiscoverManifestInWork finds manifests within an already-fetched ManifestWork.
// Use this when you already have the ManifestWork as unstructured and don't need to fetch it.
//
// Parameters:
//   - work: The ManifestWork (as *unstructured.Unstructured) to search within
//   - discovery: Discovery configuration (namespace, name, or label selector)
//
// Returns:
//   - List of matching manifests as unstructured objects
//   - The manifest with the highest generation if multiple match (use manifest.GetLatestGenerationFromList)
func (c *Client) DiscoverManifestInWork(
	work *unstructured.Unstructured,
	discovery manifest.Discovery,
) (*unstructured.UnstructuredList, error) {
	return manifest.DiscoverNestedManifest(work, discovery)
}

// isConsumerNotFoundError detects when Maestro rejects a ManifestWork create because
// the consumer (target cluster) is not registered. Maestro surfaces this as a raw
// PostgreSQL foreign-key constraint violation; we detect and sanitize it.
func isConsumerNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "fk_resources_consumers")
}

// workToUnstructured converts a typed *workv1.ManifestWork to *unstructured.Unstructured.
// The Maestro API often returns ManifestWork without apiVersion/kind, so we set the GVK explicitly.
func workToUnstructured(work *workv1.ManifestWork) (*unstructured.Unstructured, error) {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(work)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{Object: obj}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   constants.ManifestWorkGroup,
		Version: constants.ManifestWorkVersion,
		Kind:    constants.ManifestWorkKind,
	})
	return u, nil
}
