package maestroclient

import (
	"context"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/manifest"
	workv1 "open-cluster-management.io/api/work/v1"
)

// ApplyManifestWorkResult contains the result of an ApplyManifestWork operation.
type ApplyManifestWorkResult struct {
	// Work is the ManifestWork after the operation (created, updated, or existing).
	Work *workv1.ManifestWork
	// Operation is the actual operation performed (create, update, or skip).
	Operation manifest.Operation
	// Reason describes why the operation was performed.
	Reason string
}

// ManifestWorkClient defines the interface for ManifestWork operations.
// This interface enables easier testing through mocking.
type ManifestWorkClient interface {
	// CreateManifestWork creates a new ManifestWork for a target cluster (consumer)
	CreateManifestWork(
		ctx context.Context,
		consumerName string,
		work *workv1.ManifestWork,
	) (*workv1.ManifestWork, error)

	// GetManifestWork retrieves a ManifestWork by name from a target cluster
	GetManifestWork(
		ctx context.Context,
		consumerName string,
		workName string,
	) (*workv1.ManifestWork, error)

	// ApplyManifestWork creates or updates a ManifestWork (upsert operation).
	// Returns the result including the actual operation performed (create, update, or skip).
	ApplyManifestWork(
		ctx context.Context,
		consumerName string,
		work *workv1.ManifestWork,
	) (*ApplyManifestWorkResult, error)

	// DeleteManifestWork deletes a ManifestWork from a target cluster
	DeleteManifestWork(ctx context.Context, consumerName string, workName string) error

	// ListManifestWorks lists all ManifestWorks for a target cluster
	ListManifestWorks(
		ctx context.Context,
		consumerName string,
		labelSelector string,
	) (*workv1.ManifestWorkList, error)

	// PatchManifestWork patches an existing ManifestWork using JSON merge patch
	PatchManifestWork(
		ctx context.Context,
		consumerName string,
		workName string,
		patchData []byte,
	) (*workv1.ManifestWork, error)
}

// Ensure Client implements ManifestWorkClient
var _ ManifestWorkClient = (*Client)(nil)
