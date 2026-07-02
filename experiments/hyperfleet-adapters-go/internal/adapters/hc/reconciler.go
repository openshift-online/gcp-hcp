// Package hc implements the hc-adapter reconciler for managing HostedClusters via ManifestWork.
package hc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/adapters/hc/manifest"
	common "github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/hyperfleetapi"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/transport"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	adapterName = "hc-adapter"

	requeueReady      = 5 * time.Minute
	requeueNotReady   = 30 * time.Second

	// hostedClusterManifestIndex is the manifest index for the HostedCluster in the ManifestWork.
	hostedClusterManifestIndex = 3
)

// Reconciler implements the hc-adapter reconcile loop.
type Reconciler struct {
	api       hyperfleetapi.Client
	transport transport.Client
	log       logger.Logger
}

// New creates a new Reconciler.
func New(api hyperfleetapi.Client, transport transport.Client, log logger.Logger) *Reconciler {
	return &Reconciler{
		api:       api,
		transport: transport,
		log:       log,
	}
}

// Reconcile processes one cluster by ID and returns a Result indicating when to requeue.
func (r *Reconciler) Reconcile(ctx context.Context, clusterID string) (common.Result, error) {
	log := r.log.With("adapter", adapterName).With("cluster_id", clusterID)

	// Step 1: GET /clusters/{id}
	cluster, err := r.api.GetCluster(ctx, clusterID)
	if err != nil {
		var notFound *hyperfleetapi.NotFoundError
		if errors.As(err, &notFound) {
			log.Infof(ctx, "cluster not found, skipping")
			return common.Result{}, nil
		}
		return common.Result{}, fmt.Errorf("%s: get cluster: %w", adapterName, err)
	}

	// Step 2: Skip if already reconciled (Reconciled condition == "True").
	for _, cond := range cluster.Status.Conditions {
		if cond.Type == "Reconciled" && cond.Status == "True" {
			log.Infof(ctx, "cluster already reconciled, requeueing after %s", requeueReady)
			return common.Result{RequeueAfter: requeueReady}, nil
		}
	}

	// Step 3: GET /clusters/{id}/statuses
	statuses, err := r.api.GetClusterStatuses(ctx, clusterID)
	if err != nil {
		return common.Result{}, fmt.Errorf("%s: get cluster statuses: %w", adapterName, err)
	}

	// Step 4: Check placement and version-resolution readiness.
	placement := statuses.Placement()
	vr := statuses.VersionResolution()

	if !placement.Ready() || !vr.Ready() || vr.ReleaseVersion != cluster.Spec.Release.Version {
		log.Infof(ctx, "dependencies not ready (placement=%v, vr=%v), requeueing after %s",
			placement.Ready(), vr.Ready(), requeueNotReady)
		return common.Result{RequeueAfter: requeueNotReady}, nil
	}

	// Step 5: Build ManifestWork.
	mwInput := manifest.Input{
		ClusterID:                    clusterID,
		ClusterName:                  cluster.Name,
		Generation:                   cluster.Generation,
		CreatedBy:                    cluster.CreatedBy,
		InfraID:                      cluster.Spec.InfraID,
		IssuerURL:                    cluster.Spec.IssuerURL,
		ClusterIDUUID:                cluster.ID,
		GCPProjectID:                 cluster.Spec.Platform.GCP.ProjectID,
		GCPRegion:                    cluster.Spec.Platform.GCP.Region,
		GCPNetwork:                   cluster.Spec.Platform.GCP.Network,
		GCPSubnet:                    cluster.Spec.Platform.GCP.Subnet,
		GCPEndpointAccess:            cluster.Spec.Platform.GCP.EndpointAccess,
		WIFProjectNumber:             cluster.Spec.Platform.GCP.WorkloadIdentity.ProjectNumber,
		WIFPoolID:                    cluster.Spec.Platform.GCP.WorkloadIdentity.PoolID,
		WIFProviderID:                cluster.Spec.Platform.GCP.WorkloadIdentity.ProviderID,
		NodePoolEmail:                cluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsRef.NodePool,
		ControlPlaneEmail:            cluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsRef.ControlPlane,
		CloudControllerEmail:         cluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsRef.CloudController,
		StorageEmail:                 cluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsRef.Storage,
		ImageRegistryEmail:           cluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsRef.ImageRegistry,
		NetworkEmail:                 cluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsRef.Network,
		ReleaseImage:                 vr.ReleaseImage,
		ReleaseChannel:               vr.ReleaseChannel,
		BaseDomain:                   placement.BaseDomain,
	}

	mw, err := manifest.Build(mwInput)
	if err != nil {
		return common.Result{}, fmt.Errorf("%s: build manifest work: %w", adapterName, err)
	}

	// Step 6: Apply ManifestWork.
	if err := r.transport.Apply(ctx, placement.ManagementClusterName, mw); err != nil {
		return common.Result{}, fmt.Errorf("%s: apply manifest work: %w", adapterName, err)
	}

	// Step 7: Get ManifestWork status.
	mwName := mw.Name
	mwStatus, err := r.transport.GetStatus(ctx, placement.ManagementClusterName, mwName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// ManifestWork hasn't been processed yet — report Unknown conditions.
			mwStatus = nil
		} else {
			return common.Result{}, fmt.Errorf("%s: get manifest work status: %w", adapterName, err)
		}
	}

	// Step 8: Build status payload.
	payload := r.buildStatusPayload(cluster.Generation, mwStatus)

	// Step 9: PUT /clusters/{id}/statuses.
	if err := r.api.PutClusterStatus(ctx, clusterID, payload); err != nil {
		return common.Result{}, fmt.Errorf("%s: put cluster status: %w", adapterName, err)
	}

	// Step 10: Requeue after 5 minutes.
	return common.Result{RequeueAfter: requeueReady}, nil
}

// buildStatusPayload constructs the StatusPayload from the ManifestWork status.
func (r *Reconciler) buildStatusPayload(generation int64, mwStatus *transport.ManifestWorkStatus) hyperfleetapi.StatusPayload {
	payload := hyperfleetapi.StatusPayload{
		Adapter:            adapterName,
		ObservedGeneration: generation,
		ObservedTime:       time.Now().UTC().Format(time.RFC3339),
	}

	if mwStatus == nil {
		// ManifestWork not yet processed.
		payload.Conditions = []hyperfleetapi.Condition{
			{Type: "Applied", Status: "Unknown", Reason: "ManifestWorkNotFound", Message: "ManifestWork has not been processed yet"},
			{Type: "Available", Status: "Unknown", Reason: "ManifestWorkNotFound", Message: "ManifestWork has not been processed yet"},
			{Type: "Health", Status: "False", Reason: "ManifestWorkNotFound", Message: "ManifestWork has not been processed yet"},
		}
		payload.Data = map[string]any{
			"available": false,
		}
		return payload
	}

	// Derive Applied condition from top-level MW conditions.
	appliedStatus := conditionStatus(mwStatus.Conditions, "Applied")

	// Derive Available and Health from HC manifest statusFeedback (index 3).
	availableStatus := "Unknown"
	healthStatus := "False"

	if len(mwStatus.ResourceStatuses) > hostedClusterManifestIndex {
		hcFeedback := mwStatus.ResourceStatuses[hostedClusterManifestIndex]
		if v, ok := hcFeedback["availableCondition"]; ok {
			availableStatus = v
		}
		if degraded, ok := hcFeedback["degradedCondition"]; ok {
			if degraded == "False" {
				healthStatus = "True"
			} else {
				healthStatus = "False"
			}
		}
	}

	payload.Conditions = []hyperfleetapi.Condition{
		{Type: "Applied", Status: appliedStatus, Reason: "ManifestWorkApplied"},
		{Type: "Available", Status: availableStatus, Reason: "HostedClusterAvailable"},
		{Type: "Health", Status: healthStatus, Reason: "HostedClusterHealth"},
	}
	payload.Data = map[string]any{
		"available": availableStatus == "True",
	}

	return payload
}

// conditionStatus returns the status of the first condition matching condType,
// or "Unknown" if not found.
func conditionStatus(conditions []metav1.Condition, condType string) string {
	for _, c := range conditions {
		if c.Type == condType {
			return string(c.Status)
		}
	}
	return "Unknown"
}
