package maestro

import (
	"context"
	"fmt"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/maestroclient"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/transport"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	workv1 "open-cluster-management.io/api/work/v1"
)

// Client is a thin transport.Client wrapper over maestroclient.ManifestWorkClient.
type Client struct {
	mwc      maestroclient.ManifestWorkClient
	sourceID string
	log      logger.Logger
}

// Ensure Client implements transport.Client.
var _ transport.Client = (*Client)(nil)

// New creates a new maestro transport Client.
func New(mwc maestroclient.ManifestWorkClient, sourceID string, log logger.Logger) *Client {
	return &Client{
		mwc:      mwc,
		sourceID: sourceID,
		log:      log,
	}
}

// Apply creates or updates a ManifestWork on the target cluster via Maestro.
func (c *Client) Apply(ctx context.Context, targetCluster string, mw *workv1.ManifestWork) error {
	_, err := c.mwc.ApplyManifestWork(ctx, targetCluster, mw)
	if err != nil {
		return fmt.Errorf("apply ManifestWork %s/%s: %w", targetCluster, mw.GetName(), err)
	}
	return nil
}

// GetStatus reads back the ManifestWork status from Maestro and maps it into
// a transport.ManifestWorkStatus.
func (c *Client) GetStatus(ctx context.Context, targetCluster, name string) (*transport.ManifestWorkStatus, error) {
	work, err := c.mwc.GetManifestWork(ctx, targetCluster, name)
	if err != nil {
		return nil, fmt.Errorf("get ManifestWork %s/%s: %w", targetCluster, name, err)
	}

	// Map top-level conditions.
	conditions := make([]metav1.Condition, len(work.Status.Conditions))
	copy(conditions, work.Status.Conditions)

	// Map per-resource statusFeedback values.
	manifests := work.Status.ResourceStatus.Manifests
	resourceStatuses := make([]map[string]string, len(manifests))
	for i, mc := range manifests {
		m := make(map[string]string, len(mc.StatusFeedbacks.Values))
		for _, v := range mc.StatusFeedbacks.Values {
			m[v.Name] = feedbackValueToString(v.Value)
		}
		resourceStatuses[i] = m
	}

	return &transport.ManifestWorkStatus{
		Conditions:       conditions,
		ResourceStatuses: resourceStatuses,
	}, nil
}

// Delete removes the ManifestWork from the target cluster. Not-found errors are ignored.
func (c *Client) Delete(ctx context.Context, targetCluster, name string) error {
	err := c.mwc.DeleteManifestWork(ctx, targetCluster, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete ManifestWork %s/%s: %w", targetCluster, name, err)
	}
	return nil
}

// feedbackValueToString converts a workv1.FieldValue to a human-readable string.
func feedbackValueToString(fv workv1.FieldValue) string {
	switch fv.Type {
	case workv1.String:
		if fv.String != nil {
			return *fv.String
		}
	case workv1.Integer:
		if fv.Integer != nil {
			return fmt.Sprintf("%d", *fv.Integer)
		}
	case workv1.Boolean:
		if fv.Boolean != nil {
			return fmt.Sprintf("%t", *fv.Boolean)
		}
	case workv1.JsonRaw:
		if fv.JsonRaw != nil {
			return *fv.JsonRaw
		}
	}
	return ""
}
