package transport

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	workv1 "open-cluster-management.io/api/work/v1"
)

// ManifestWorkStatus holds the status read back from Maestro after applying a ManifestWork.
type ManifestWorkStatus struct {
	// Conditions contains the top-level ManifestWork conditions (Applied, Available, etc.).
	Conditions []metav1.Condition
	// ResourceStatuses contains statusFeedback values keyed by manifest index then by name.
	// e.g. ResourceStatuses[0]["availableCondition"] = "True"
	ResourceStatuses []map[string]string
}

// Client abstracts the transport layer (Maestro today, Firestore in future).
type Client interface {
	// Apply creates or updates a ManifestWork on the target cluster.
	Apply(ctx context.Context, targetCluster string, mw *workv1.ManifestWork) error
	// GetStatus reads back the ManifestWork status.
	GetStatus(ctx context.Context, targetCluster, name string) (*ManifestWorkStatus, error)
	// Delete removes the ManifestWork from the target cluster.
	Delete(ctx context.Context, targetCluster, name string) error
}
