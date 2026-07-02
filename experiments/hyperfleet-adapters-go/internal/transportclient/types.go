// Package transportclient provides a unified interface for applying Kubernetes resources
// across different backends (direct K8s API, Maestro/OCM ManifestWork, etc.).
package transportclient

import (
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/manifest"
)

// ApplyOptions configures the behavior of resource apply operations.
type ApplyOptions struct {
	// RecreateOnChange forces delete+create instead of update when resource exists
	// and generation has changed. Useful for resources that don't support in-place updates.
	RecreateOnChange bool
}

// DeleteOptions configures the behavior of resource delete operations.
type DeleteOptions struct {
	// PropagationPolicy is the Kubernetes deletion propagation policy.
	// Valid values: "Background" (default), "Foreground", "Orphan".
	// Ignored for Maestro transport — ManifestWork handles its own cleanup semantics.
	PropagationPolicy string
}

// ApplyResult contains the result of applying a single resource.
type ApplyResult struct {
	// Operation is the operation that was performed (create, update, recreate, skip)
	Operation manifest.Operation

	// Reason explains why the operation was chosen
	Reason string
}

// TransportContext carries per-request routing information for the transport backend.
// Each transport client defines its own concrete context type and type-asserts:
//   - k8sclient: ignores it (nil)
//   - maestroclient: expects *maestroclient.TransportContext with ConsumerName
//
// This is typed as `any` to allow each backend to define its own context shape.
type TransportContext = any
