package placement

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/hyperfleetapi"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
)

const (
	adapterName  = "placement-adapter"
	requeueAfter = 5 * time.Minute
)

// Reconciler implements the placement adapter reconcile loop.
type Reconciler struct {
	hfClient   hyperfleetapi.Client
	selector   Selector
	candidates []Candidate
	log        logger.Logger
}

// NewReconciler creates a new placement Reconciler.
func NewReconciler(hfClient hyperfleetapi.Client, selector Selector, candidates []Candidate, log logger.Logger) *Reconciler {
	return &Reconciler{
		hfClient:   hfClient,
		selector:   selector,
		candidates: candidates,
		log:        log,
	}
}

// Reconcile runs the placement reconciliation loop for the given cluster ID.
func (r *Reconciler) Reconcile(ctx context.Context, clusterID string) (common.Result, error) {
	// Step 1: GET /clusters/{id}
	cluster, err := r.hfClient.GetCluster(ctx, clusterID)
	if err != nil {
		var notFound *hyperfleetapi.NotFoundError
		if errors.As(err, &notFound) {
			r.log.Infof(ctx, "placement: cluster %s not found, skipping", clusterID)
			return common.Result{}, nil
		}
		return common.Result{}, fmt.Errorf("placement: get cluster %s: %w", clusterID, err)
	}

	// Step 2: If cluster has Reconciled condition "True" → skip
	for _, c := range cluster.Status.Conditions {
		if c.Type == "Reconciled" && c.Status == "True" {
			r.log.Infof(ctx, "placement: cluster %s already reconciled, requeuing", clusterID)
			return common.Result{RequeueAfter: requeueAfter}, nil
		}
	}

	// Step 3: GET /clusters/{id}/statuses
	statuses, err := r.hfClient.GetClusterStatuses(ctx, clusterID)
	if err != nil {
		return common.Result{}, fmt.Errorf("placement: get cluster statuses for %s: %w", clusterID, err)
	}

	placement := statuses.Placement()
	if placement.Ready() {
		r.log.Infof(ctx, "placement: cluster %s already placed (mc=%s, domain=%s), requeuing",
			clusterID, placement.ManagementClusterName, placement.BaseDomain)
		return common.Result{RequeueAfter: requeueAfter}, nil
	}

	// Step 4: Select MC and DNS zone
	mc, domain, err := r.selector.Select(ctx, r.candidates)
	if err != nil {
		return common.Result{}, fmt.Errorf("placement: select MC for cluster %s: %w", clusterID, err)
	}

	r.log.Infof(ctx, "placement: cluster %s: selected MC %s, domain %s", clusterID, mc, domain)

	// Step 5: PUT /clusters/{id}/statuses
	payload := hyperfleetapi.StatusPayload{
		Adapter:            adapterName,
		ObservedGeneration: cluster.Generation,
		ObservedTime:       time.Now().UTC().Format(time.RFC3339),
		Conditions: []hyperfleetapi.Condition{
			{
				Type:    "Applied",
				Status:  "True",
				Reason:  "PlacementDecided",
				Message: "MC and DNS zone selected",
			},
			{
				Type:    "Available",
				Status:  "True",
				Reason:  "PlacementReady",
				Message: fmt.Sprintf("Management cluster: %s, base domain: %s", mc, domain),
			},
			{
				Type:    "Health",
				Status:  "True",
				Reason:  "PlacementReady",
				Message: fmt.Sprintf("Management cluster: %s, base domain: %s", mc, domain),
			},
		},
		Data: map[string]any{
			"managementClusterName": mc,
			"baseDomain":            domain,
		},
	}

	if err := r.hfClient.PutClusterStatus(ctx, clusterID, payload); err != nil {
		return common.Result{}, fmt.Errorf("placement: put cluster status for %s: %w", clusterID, err)
	}

	// Step 6: Requeue
	return common.Result{RequeueAfter: requeueAfter}, nil
}
