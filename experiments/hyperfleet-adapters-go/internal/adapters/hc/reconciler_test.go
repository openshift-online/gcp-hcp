package hc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/adapters/hc"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/hyperfleetapi"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/transport"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/transport/mock"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const apiVersion = "v1"

// testLogger returns a no-op logger for tests.
func testLogger(t *testing.T) logger.Logger {
	t.Helper()
	log, err := logger.NewLogger(logger.Config{
		Level:     "error",
		Format:    "text",
		Output:    "stderr",
		Component: "test",
	})
	require.NoError(t, err)
	return log
}

// clusterDetail builds a standard ClusterDetail for tests.
func clusterDetail(name string, conditions []hyperfleetapi.Condition) hyperfleetapi.ClusterDetail {
	return hyperfleetapi.ClusterDetail{
		ID:         "cluster-abc",
		Name:       name,
		Generation: 2,
		CreatedBy:  "alice@redhat.com",
		Spec: hyperfleetapi.ClusterSpec{
			InfraID:   "infra-xyz",
			IssuerURL: "https://issuer.example.com",
			ClusterID: "550e8400-e29b-41d4-a716-446655440000",
			Release:   hyperfleetapi.ReleaseSpec{Version: "4.15.0"},
			Platform: hyperfleetapi.GCPPlatform{
				Type: "GCP",
				GCP: hyperfleetapi.GCPConfig{
					ProjectID: "my-project",
					Region:    "us-central1",
					Network:   "my-vpc",
					Subnet:    "my-subnet",
					WorkloadIdentity: hyperfleetapi.WIFConfig{
						ProjectNumber: "12345",
						PoolID:        "pool",
						ProviderID:    "provider",
						ServiceAccountsRef: hyperfleetapi.WIFServiceAccounts{
							NodePool:        "np@sa.iam.gserviceaccount.com",
							ControlPlane:    "cp@sa.iam.gserviceaccount.com",
							CloudController: "cc@sa.iam.gserviceaccount.com",
							Storage:         "st@sa.iam.gserviceaccount.com",
							ImageRegistry:   "ir@sa.iam.gserviceaccount.com",
							Network:         "nw@sa.iam.gserviceaccount.com",
						},
					},
				},
			},
		},
		Status: hyperfleetapi.ClusterStatus{
			Conditions: conditions,
		},
	}
}

// readyStatuses returns AdapterStatuses with placement and VR data ready.
func readyStatuses(version string) hyperfleetapi.AdapterStatuses {
	return hyperfleetapi.AdapterStatuses{
		{
			Adapter: "placement-adapter",
			Data: map[string]any{
				"managementClusterName": "mc-cluster-1",
				"baseDomain":            "example.com",
			},
		},
		{
			Adapter: "version-resolution-adapter",
			Data: map[string]any{
				"release_image":   "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
				"release_version": version,
				"release_channel": "stable-4.15",
			},
		},
	}
}

// setupServer creates an httptest server with the given handler map (path -> handler).
// Paths are matched by prefix.
func setupServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, handler := range handlers {
		mux.HandleFunc(path, handler)
	}
	return httptest.NewServer(mux)
}

func jsonResponse(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(v))
}

// TestReconcile_HappyPath verifies the full reconcile path when all dependencies are ready.
func TestReconcile_HappyPath(t *testing.T) {
	ctx := context.Background()
	clusterID := "cluster-abc"
	mwName := clusterID + "-hc-adapter"
	mcName := "mc-cluster-1"

	detail := clusterDetail("my-cluster", nil)
	statuses := readyStatuses("4.15.0")

	putCalled := false

	server := setupServer(t, map[string]http.HandlerFunc{
		"/api/hyperfleet/v1/clusters/" + clusterID: func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && !strings.HasSuffix(r.URL.Path, "/statuses") {
				jsonResponse(t, w, detail)
			}
		},
		"/api/hyperfleet/v1/clusters/" + clusterID + "/statuses": func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				jsonResponse(t, w, map[string]any{"items": statuses})
			case http.MethodPut:
				putCalled = true
				w.WriteHeader(http.StatusNoContent)
			}
		},
	})
	defer server.Close()

	apiClient := hyperfleetapi.New(server.URL, apiVersion, testLogger(t))
	mockTransport := mock.New()

	// Inject a ManifestWork status with Applied + Available conditions.
	applied := metav1.ConditionTrue
	appliedCond := metav1.Condition{Type: "Applied", Status: metav1.ConditionStatus(applied), LastTransitionTime: metav1.Now()}
	mockTransport.StatusOverrides[mcName+"/"+mwName] = &transport.ManifestWorkStatus{
		Conditions: []metav1.Condition{appliedCond},
		ResourceStatuses: []map[string]string{
			{}, {}, {}, // indices 0-2
			{"availableCondition": "True", "degradedCondition": "False"}, // index 3 = HC
			{}, // index 4 = Job
		},
	}

	r := hc.New(apiClient, mockTransport, testLogger(t))
	result, err := r.Reconcile(ctx, clusterID)
	require.NoError(t, err)
	require.Equal(t, 5*time.Minute, result.RequeueAfter)

	// Verify Apply was called.
	require.Len(t, mockTransport.ApplyCalls, 1)
	require.Equal(t, mcName, mockTransport.ApplyCalls[0].TargetCluster)
	require.Equal(t, mwName, mockTransport.ApplyCalls[0].Work.Name)

	// Verify PUT status was called.
	require.True(t, putCalled, "expected PutClusterStatus to be called")
}

// TestReconcile_DependenciesNotReady_NoPlacement verifies requeue when placement is missing.
func TestReconcile_DependenciesNotReady_NoPlacement(t *testing.T) {
	ctx := context.Background()
	clusterID := "cluster-abc"
	detail := clusterDetail("my-cluster", nil)

	// Statuses without placement adapter.
	statusesNoPlacement := hyperfleetapi.AdapterStatuses{
		{
			Adapter: "version-resolution-adapter",
			Data: map[string]any{
				"release_image":   "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
				"release_version": "4.15.0",
				"release_channel": "stable-4.15",
			},
		},
	}

	server := setupServer(t, map[string]http.HandlerFunc{
		"/api/hyperfleet/v1/clusters/" + clusterID: func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/statuses") {
				jsonResponse(t, w, detail)
			}
		},
		"/api/hyperfleet/v1/clusters/" + clusterID + "/statuses": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				jsonResponse(t, w, map[string]any{"items": statusesNoPlacement})
			}
		},
	})
	defer server.Close()

	apiClient := hyperfleetapi.New(server.URL, apiVersion, testLogger(t))
	mockTransport := mock.New()

	r := hc.New(apiClient, mockTransport, testLogger(t))
	result, err := r.Reconcile(ctx, clusterID)
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, result.RequeueAfter)

	// Apply should NOT have been called.
	require.Empty(t, mockTransport.ApplyCalls)
}

// TestReconcile_DependenciesNotReady_VRVersionMismatch verifies requeue when VR version doesn't match.
func TestReconcile_DependenciesNotReady_VRVersionMismatch(t *testing.T) {
	ctx := context.Background()
	clusterID := "cluster-abc"

	// Cluster wants 4.15.0 but VR resolved 4.14.9.
	detail := clusterDetail("my-cluster", nil)
	statusesWrongVR := readyStatuses("4.14.9") // mismatch

	server := setupServer(t, map[string]http.HandlerFunc{
		"/api/hyperfleet/v1/clusters/" + clusterID: func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/statuses") {
				jsonResponse(t, w, detail)
			}
		},
		"/api/hyperfleet/v1/clusters/" + clusterID + "/statuses": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				jsonResponse(t, w, map[string]any{"items": statusesWrongVR})
			}
		},
	})
	defer server.Close()

	apiClient := hyperfleetapi.New(server.URL, apiVersion, testLogger(t))
	mockTransport := mock.New()

	r := hc.New(apiClient, mockTransport, testLogger(t))
	result, err := r.Reconcile(ctx, clusterID)
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, result.RequeueAfter)

	require.Empty(t, mockTransport.ApplyCalls)
}

// TestReconcile_ClusterNotFound verifies that a 404 returns empty Result with no error.
func TestReconcile_ClusterNotFound(t *testing.T) {
	ctx := context.Background()
	clusterID := "cluster-missing"

	server := setupServer(t, map[string]http.HandlerFunc{
		"/api/hyperfleet/v1/clusters/" + clusterID: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	})
	defer server.Close()

	apiClient := hyperfleetapi.New(server.URL, apiVersion, testLogger(t))
	mockTransport := mock.New()

	r := hc.New(apiClient, mockTransport, testLogger(t))
	result, err := r.Reconcile(ctx, clusterID)
	require.NoError(t, err)
	require.Zero(t, result.RequeueAfter)
}

// TestReconcile_AlreadyReconciled verifies that a cluster with Reconciled=True is skipped.
func TestReconcile_AlreadyReconciled(t *testing.T) {
	ctx := context.Background()
	clusterID := "cluster-abc"

	reconciledConditions := []hyperfleetapi.Condition{
		{Type: "Reconciled", Status: "True", Reason: "Done"},
	}
	detail := clusterDetail("my-cluster", reconciledConditions)

	server := setupServer(t, map[string]http.HandlerFunc{
		"/api/hyperfleet/v1/clusters/" + clusterID: func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/statuses") {
				jsonResponse(t, w, detail)
			}
		},
	})
	defer server.Close()

	apiClient := hyperfleetapi.New(server.URL, apiVersion, testLogger(t))
	mockTransport := mock.New()

	r := hc.New(apiClient, mockTransport, testLogger(t))
	result, err := r.Reconcile(ctx, clusterID)
	require.NoError(t, err)
	require.Equal(t, 5*time.Minute, result.RequeueAfter)

	// No Apply or GetStatus should have been called.
	require.Empty(t, mockTransport.ApplyCalls)
}
