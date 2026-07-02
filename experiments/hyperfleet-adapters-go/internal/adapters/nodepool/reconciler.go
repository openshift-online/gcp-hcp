// Package nodepool implements the nodepool adapter reconciler.
package nodepool

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/adapters/nodepool/manifest"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/hyperfleetapi"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/transport"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
)

const (
	adapterName        = "nodepool-adapter"
	requeueNotReady    = 30 * time.Second
	requeueAfterApply  = 5 * time.Minute
)

// Reconciler implements the nodepool adapter reconciliation loop.
type Reconciler struct {
	api       hyperfleetapi.Client
	transport transport.Client
	log       logger.Logger
}

// New creates a new nodepool Reconciler.
func New(api hyperfleetapi.Client, transport transport.Client, log logger.Logger) *Reconciler {
	return &Reconciler{
		api:       api,
		transport: transport,
		log:       log,
	}
}

// Reconcile runs one reconciliation cycle for the given nodepoolID.
func (r *Reconciler) Reconcile(ctx context.Context, nodepoolID string) (common.Result, error) {
	log := r.log.With("nodepoolID", nodepoolID)

	// Step 1: GET nodepool
	np, err := r.api.GetNodePool(ctx, nodepoolID)
	if err != nil {
		var nfe *hyperfleetapi.NotFoundError
		if errors.As(err, &nfe) {
			log.Infof(ctx, "nodepool %s not found, skipping", nodepoolID)
			return common.Result{}, nil
		}
		return common.Result{}, fmt.Errorf("nodepool reconciler: get nodepool: %w", err)
	}

	// Step 2: GET cluster
	cluster, err := r.api.GetCluster(ctx, np.ClusterID)
	if err != nil {
		var nfe *hyperfleetapi.NotFoundError
		if errors.As(err, &nfe) {
			log.Infof(ctx, "cluster %s not found for nodepool %s, skipping", np.ClusterID, nodepoolID)
			return common.Result{}, nil
		}
		return common.Result{}, fmt.Errorf("nodepool reconciler: get cluster: %w", err)
	}

	// Step 3: GET cluster statuses
	clusterStatuses, err := r.api.GetClusterStatuses(ctx, np.ClusterID)
	if err != nil {
		return common.Result{}, fmt.Errorf("nodepool reconciler: get cluster statuses: %w", err)
	}

	// Step 4: GET nodepool statuses
	nodepoolStatuses, err := r.api.GetNodePoolStatuses(ctx, nodepoolID)
	if err != nil {
		return common.Result{}, fmt.Errorf("nodepool reconciler: get nodepool statuses: %w", err)
	}

	// Step 5: Gate check
	placement := clusterStatuses.Placement()
	hc := clusterStatuses.HCAdapter()
	nodepoolVR := nodepoolStatuses.NodePoolVR()

	if !placement.Ready() {
		log.Infof(ctx, "placement not ready for nodepool %s, requeueing", nodepoolID)
		return common.Result{RequeueAfter: requeueNotReady}, nil
	}
	if !hc.Available() {
		log.Infof(ctx, "hc-adapter not available for nodepool %s, requeueing", nodepoolID)
		return common.Result{RequeueAfter: requeueNotReady}, nil
	}
	if !nodepoolVR.Ready() {
		log.Infof(ctx, "nodepool VR not ready for nodepool %s, requeueing", nodepoolID)
		return common.Result{RequeueAfter: requeueNotReady}, nil
	}
	if nodepoolVR.ReleaseVersion != np.Spec.Release.Version {
		log.Infof(ctx, "nodepool VR version %q does not match spec version %q for nodepool %s, requeueing",
			nodepoolVR.ReleaseVersion, np.Spec.Release.Version, nodepoolID)
		return common.Result{RequeueAfter: requeueNotReady}, nil
	}

	// Step 6: Build ManifestWork
	zone := np.Spec.Platform.GCP.Zone
	if zone == "" {
		zone = np.Spec.Platform.GCP.Region + "-a"
	}

	_ = cluster // cluster fetched for validation; subnet/region come from nodepool spec

	mw, err := manifest.Build(manifest.Input{
		NodePoolID:         nodepoolID,
		NodePoolName:       np.Name,
		NodePoolGeneration: np.Generation,
		ClusterID:          np.ClusterID,
		ClusterName:        cluster.Name,
		Replicas:           defaultReplicas,
		MachineType:        manifest.DefaultMachineType,
		GCPRegion:          np.Spec.Platform.GCP.Region,
		Zone:               zone,
		GCPSubnet:          cluster.Spec.Platform.GCP.Subnet,
		DiskSizeGB:         manifest.DefaultDiskSizeGB,
		DiskType:           manifest.DefaultDiskType,
		ReleaseImage:       nodepoolVR.ReleaseImage,
	})
	if err != nil {
		return common.Result{}, fmt.Errorf("nodepool reconciler: build manifest work: %w", err)
	}

	managementCluster := placement.ManagementClusterName
	mwName := fmt.Sprintf("%s-%s", nodepoolID, adapterName)

	// Step 7: Apply ManifestWork
	if err := r.transport.Apply(ctx, managementCluster, mw); err != nil {
		return common.Result{}, fmt.Errorf("nodepool reconciler: apply manifest work: %w", err)
	}

	// Step 8: GetStatus
	mwStatus, err := r.transport.GetStatus(ctx, managementCluster, mwName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Infof(ctx, "manifest work %s not found yet, reporting unknown status", mwName)
			mwStatus = nil
		} else {
			return common.Result{}, fmt.Errorf("nodepool reconciler: get manifest work status: %w", err)
		}
	}

	// Step 9: Build status payload
	payload := r.buildStatusPayload(np, mwStatus)

	// Step 10: PUT nodepool status
	if err := r.api.PutNodePoolStatus(ctx, nodepoolID, payload); err != nil {
		return common.Result{}, fmt.Errorf("nodepool reconciler: put nodepool status: %w", err)
	}

	// Step 11: Requeue after 5m
	return common.Result{RequeueAfter: requeueAfterApply}, nil
}

// buildStatusPayload constructs the StatusPayload from the ManifestWorkStatus.
func (r *Reconciler) buildStatusPayload(np *hyperfleetapi.NodePoolDetail, mwStatus *transport.ManifestWorkStatus) hyperfleetapi.StatusPayload {
	now := time.Now().UTC().Format(time.RFC3339)

	if mwStatus == nil {
		return hyperfleetapi.StatusPayload{
			Adapter:            adapterName,
			ObservedGeneration: np.Generation,
			ObservedTime:       now,
			Conditions: []hyperfleetapi.Condition{
				{Type: "Applied", Status: "Unknown", Reason: "ManifestWorkNotFound"},
				{Type: "Available", Status: "Unknown", Reason: "ManifestWorkNotFound"},
				{Type: "Health", Status: "Unknown", Reason: "ManifestWorkNotFound"},
			},
			Data: map[string]any{
				"replicas": "",
				"version":  "",
			},
		}
	}

	// Extract conditions from top-level ManifestWork conditions
	appliedStatus := "Unknown"
	appliedReason := "Unknown"
	for _, c := range mwStatus.Conditions {
		if c.Type == "Applied" {
			appliedStatus = string(c.Status)
			appliedReason = c.Reason
			break
		}
	}

	// Extract resource status from manifest index 0 (the NodePool)
	availableStatus := "Unknown"
	allNodesHealthy := "Unknown"
	replicas := ""
	version := ""

	if len(mwStatus.ResourceStatuses) > 0 {
		rs := mwStatus.ResourceStatuses[0]
		if v, ok := rs["readyCondition"]; ok {
			availableStatus = v
		}
		if v, ok := rs["allNodesHealthyCondition"]; ok {
			allNodesHealthy = v
		}
		if v, ok := rs["replicas"]; ok {
			replicas = v
		}
		if v, ok := rs["version"]; ok {
			version = v
		}
	}

	healthStatus := "False"
	if allNodesHealthy == "True" {
		healthStatus = "True"
	}

	conditions := []hyperfleetapi.Condition{
		{
			Type:   "Applied",
			Status: appliedStatus,
			Reason: appliedReason,
		},
		{
			Type:   "Available",
			Status: availableStatus,
		},
		{
			Type:   "Health",
			Status: healthStatus,
		},
	}

	return hyperfleetapi.StatusPayload{
		Adapter:            adapterName,
		ObservedGeneration: np.Generation,
		ObservedTime:       now,
		Conditions:         conditions,
		Data: map[string]any{
			"replicas": replicas,
			"version":  version,
		},
	}
}

// defaultReplicas is the hardcoded default for this POC.
const defaultReplicas = int32(1)

// conditionStatus converts a metav1.ConditionStatus to string.
func conditionStatusString(s metav1.ConditionStatus) string {
	return string(s)
}

var _ = conditionStatusString // suppress unused warning
