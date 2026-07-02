package nodepoolvrresolution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/hyperfleetapi"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/adapters/versionresolution"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
)

const (
	adapterName         = "nodepool-vr-adapter"
	defaultChannelGroup = "candidate"
	requeueShort        = 30 * time.Second
	requeueLong         = 5 * time.Minute
)

// Reconciler resolves the OCP release image for a node pool via Cincinnati.
type Reconciler struct {
	hfClient   hyperfleetapi.Client
	cincinnati *versionresolution.CincinnatiClient
	log        logger.Logger
}

// NewReconciler creates a new nodepool-vr Reconciler.
func NewReconciler(hfClient hyperfleetapi.Client, cincinnati *versionresolution.CincinnatiClient, log logger.Logger) *Reconciler {
	return &Reconciler{
		hfClient:   hfClient,
		cincinnati: cincinnati,
		log:        log,
	}
}

// Reconcile implements the nodepool version-resolution adapter reconciliation loop.
func (r *Reconciler) Reconcile(ctx context.Context, nodepoolID string) (common.Result, error) {
	// Step 1: GET /nodepools/{id}
	np, err := r.hfClient.GetNodePool(ctx, nodepoolID)
	if err != nil {
		var notFound *hyperfleetapi.NotFoundError
		if errors.As(err, &notFound) {
			r.log.Infof(ctx, "nodepool-vr: nodepool %s: not found, skipping", nodepoolID)
			return common.Result{}, nil
		}
		return common.Result{}, fmt.Errorf("nodepool-vr: get nodepool %s: %w", nodepoolID, err)
	}

	// Step 2: If version is empty, wait for it to be set.
	version := np.Spec.Release.Version
	if version == "" {
		r.log.Infof(ctx, "nodepool-vr: nodepool %s: release version not set, requeueing in %s", nodepoolID, requeueShort)
		return common.Result{RequeueAfter: requeueShort}, nil
	}

	// Step 3: GET /nodepools/{id}/statuses and check if already resolved.
	statuses, err := r.hfClient.GetNodePoolStatuses(ctx, nodepoolID)
	if err != nil {
		return common.Result{}, fmt.Errorf("nodepool-vr: get nodepool statuses %s: %w", nodepoolID, err)
	}
	vr := statuses.NodePoolVR()
	if vr.Ready() && vr.ReleaseVersion == version {
		r.log.Debugf(ctx, "nodepool-vr: nodepool %s: version %s already resolved, requeueing in %s", nodepoolID, version, requeueLong)
		return common.Result{RequeueAfter: requeueLong}, nil
	}

	// Step 4: Resolve version via Cincinnati.
	channel, err := buildChannel(version, defaultChannelGroup)
	if err != nil {
		return common.Result{}, fmt.Errorf("nodepool-vr: build channel for nodepool %s: %w", nodepoolID, err)
	}
	r.log.Infof(ctx, "nodepool-vr: nodepool %s: resolving version %s via channel %s", nodepoolID, version, channel)

	info, err := r.cincinnati.Resolve(ctx, version, channel)
	if err != nil {
		return common.Result{}, fmt.Errorf("nodepool-vr: cincinnati resolve for nodepool %s: %w", nodepoolID, err)
	}
	if info == nil {
		r.log.Warnf(ctx, "nodepool-vr: nodepool %s: version %s not found in Cincinnati", nodepoolID, version)
		return common.Result{RequeueAfter: requeueShort}, nil
	}

	// Step 5: PUT /nodepools/{id}/statuses
	payload := hyperfleetapi.StatusPayload{
		Adapter:            adapterName,
		ObservedGeneration: np.Generation,
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
				Reason:  "ReleaseImageAvailable",
				Message: fmt.Sprintf("Release image available: %s", info.Payload),
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
			"release_version":       version,
			"release_channel":       channel,
			"release_channel_group": defaultChannelGroup,
		},
	}

	if err := r.hfClient.PutNodePoolStatus(ctx, nodepoolID, payload); err != nil {
		return common.Result{}, fmt.Errorf("nodepool-vr: put nodepool status %s: %w", nodepoolID, err)
	}

	r.log.Infof(ctx, "nodepool-vr: nodepool %s: resolved version %s", nodepoolID, version)
	return common.Result{RequeueAfter: requeueLong}, nil
}

// buildChannel constructs the Cincinnati channel name from a version string and channel group.
// e.g. "4.22.0-ec.4" + "candidate" → "candidate-4.22"
func buildChannel(version, channelGroup string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid version string %q: expected at least major.minor", version)
	}
	major := parts[0]
	minor := parts[1]
	return fmt.Sprintf("%s-%s.%s", channelGroup, major, minor), nil
}
