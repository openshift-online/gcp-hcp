package versionresolution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/hyperfleetapi"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
)

const (
	adapterName         = "version-resolution-adapter"
	defaultChannelGroup = "candidate"
	requeueLong         = 5 * time.Minute
)

// Reconciler resolves the OCP release image for a cluster via Cincinnati.
type Reconciler struct {
	hfClient   hyperfleetapi.Client
	cincinnati *CincinnatiClient
	log        logger.Logger
}

// NewReconciler creates a new version-resolution Reconciler.
func NewReconciler(hfClient hyperfleetapi.Client, cincinnati *CincinnatiClient, log logger.Logger) *Reconciler {
	return &Reconciler{
		hfClient:   hfClient,
		cincinnati: cincinnati,
		log:        log,
	}
}

// Reconcile implements the version-resolution adapter reconciliation loop.
func (r *Reconciler) Reconcile(ctx context.Context, clusterID string) (common.Result, error) {
	// Step 1: GET /clusters/{id}
	cluster, err := r.hfClient.GetCluster(ctx, clusterID)
	if err != nil {
		var notFound *hyperfleetapi.NotFoundError
		if errors.As(err, &notFound) {
			r.log.Infof(ctx, "vr: cluster %s: not found, skipping", clusterID)
			return common.Result{}, nil
		}
		return common.Result{}, fmt.Errorf("vr: get cluster %s: %w", clusterID, err)
	}

	// Step 2: If cluster has Reconciled condition == "True", skip — Sentinel will re-trigger if needed.
	for _, cond := range cluster.Status.Conditions {
		if cond.Type == "Reconciled" && cond.Status == "True" {
			r.log.Infof(ctx, "vr: cluster %s: already reconciled, waiting for next event", clusterID)
			return common.Result{}, nil
		}
	}

	// Step 3: If version is empty, wait for it to be set.
	version := cluster.Spec.Release.Version
	if version == "" {
		r.log.Infof(ctx, "vr: cluster %s: release version not set, waiting for next event", clusterID)
		return common.Result{}, nil
	}

	// Step 4: GET /clusters/{id}/statuses and check if already resolved.
	statuses, err := r.hfClient.GetClusterStatuses(ctx, clusterID)
	if err != nil {
		return common.Result{}, fmt.Errorf("vr: get cluster statuses %s: %w", clusterID, err)
	}
	vr := statuses.VersionResolution()
	if vr.Ready() && vr.ReleaseVersion == version {
		r.log.Infof(ctx, "vr: cluster %s: version %s already resolved, waiting for next event", clusterID, version)
		return common.Result{}, nil
	}

	// Step 5: Resolve version via Cincinnati.
	channel := buildChannel(version)
	r.log.Infof(ctx, "vr: cluster %s: resolving version %s via channel %s", clusterID, version, channel)

	info, err := r.cincinnati.Resolve(ctx, version, channel)
	if err != nil {
		return common.Result{}, fmt.Errorf("vr: cincinnati resolve for cluster %s: %w", clusterID, err)
	}
	if info == nil {
		r.log.Warnf(ctx, "vr: cluster %s: version %s not found in Cincinnati, waiting for next event", clusterID, version)
		return common.Result{}, nil
	}

	// Step 6: PUT /clusters/{id}/statuses
	payload := hyperfleetapi.StatusPayload{
		Adapter:            adapterName,
		ObservedGeneration: cluster.Generation,
		ObservedTime:       time.Now().UTC().Format(time.RFC3339),
		Conditions: []hyperfleetapi.Condition{
			{
				Type:    "Applied",
				Status:  "True",
				Reason:  "VersionResolved",
				Message: fmt.Sprintf("Version %s resolved", version),
			},
			{
				Type:    "Available",
				Status:  "True",
				Reason:  "VersionResolved",
				Message: fmt.Sprintf("Version %s resolved", version),
			},
			{
				Type:    "Health",
				Status:  "True",
				Reason:  "VersionResolved",
				Message: fmt.Sprintf("Version %s resolved", version),
			},
		},
		Data: map[string]any{
			"release_image":         info.Payload,
			"release_version":       info.Version,
			"release_channel":       channel,
			"release_channel_group": defaultChannelGroup,
		},
	}

	if err := r.hfClient.PutClusterStatus(ctx, clusterID, payload); err != nil {
		return common.Result{}, fmt.Errorf("vr: put cluster status %s: %w", clusterID, err)
	}

	r.log.Infof(ctx, "vr: cluster %s: resolved version %s", clusterID, version)
	return common.Result{RequeueAfter: requeueLong}, nil
}

// buildChannel constructs the Cincinnati channel name from a version string.
// e.g. "4.22.0-ec.4" → "candidate-4.22"
func buildChannel(version string) string {
	parts := strings.Split(version, ".")
	major := "4"
	minor := "0"
	if len(parts) >= 1 {
		major = parts[0]
	}
	if len(parts) >= 2 {
		minor = parts[1]
	}
	return fmt.Sprintf("%s-%s.%s", defaultChannelGroup, major, minor)
}
