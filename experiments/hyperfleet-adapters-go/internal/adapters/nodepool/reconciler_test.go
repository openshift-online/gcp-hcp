package nodepool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/hyperfleetapi"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/transport"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/transport/mock"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestLogger(t *testing.T) logger.Logger {
	t.Helper()
	log, err := logger.NewLogger(logger.Config{
		Level:     "debug",
		Format:    logger.FormatText,
		Output:    "stdout",
		Component: "test",
		Version:   "test",
	})
	require.NoError(t, err)
	return log
}

func newTestAPIClient(t *testing.T, server *httptest.Server) hyperfleetapi.Client {
	t.Helper()
	log := newTestLogger(t)
	c := hyperfleetapi.New(server.URL, "v1", log)
	return c
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v) //nolint:errcheck
	}
}

// buildTestServer creates an httptest.Server that serves the given NodePool, Cluster,
// cluster statuses, and nodepool statuses.
func buildTestServer(
	t *testing.T,
	np *hyperfleetapi.NodePoolDetail,
	cluster *hyperfleetapi.ClusterDetail,
	clusterStatuses hyperfleetapi.AdapterStatuses,
	nodepoolStatuses hyperfleetapi.AdapterStatuses,
) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/hyperfleet/v1/nodepools/"+np.ID:
			writeJSON(w, http.StatusOK, np)
		case r.Method == http.MethodGet && r.URL.Path == "/api/hyperfleet/v1/clusters/"+np.ClusterID:
			if cluster == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, cluster)
		case r.Method == http.MethodGet && r.URL.Path == "/api/hyperfleet/v1/clusters/"+np.ClusterID+"/statuses":
			writeJSON(w, http.StatusOK, map[string]any{"items": clusterStatuses})
		case r.Method == http.MethodGet && r.URL.Path == "/api/hyperfleet/v1/nodepools/"+np.ID+"/statuses":
			writeJSON(w, http.StatusOK, map[string]any{"items": nodepoolStatuses})
		case r.Method == http.MethodPut && r.URL.Path == "/api/hyperfleet/v1/nodepools/"+np.ID+"/statuses":
			w.WriteHeader(http.StatusOK)
		default:
			t.Logf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return srv
}

// fixedConditionStatus is a helper string constant for "True"
const condTrue = "True"

// buildReadyStatuses creates cluster and nodepool statuses that pass all gates.
func buildReadyStatuses(specVersion string) (hyperfleetapi.AdapterStatuses, hyperfleetapi.AdapterStatuses) {
	clusterStatuses := hyperfleetapi.AdapterStatuses{
		{
			Adapter: "placement-adapter",
			Data: map[string]any{
				"managementClusterName": "mc-us-c1",
				"baseDomain":            "hc.example.com",
			},
		},
		{
			Adapter: "hc-adapter",
			Conditions: []hyperfleetapi.Condition{
				{Type: "Available", Status: condTrue},
			},
		},
	}
	nodepoolStatuses := hyperfleetapi.AdapterStatuses{
		{
			Adapter: "nodepool-vr-adapter",
			Data: map[string]any{
				"release_image":   "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
				"release_version": specVersion,
			},
		},
	}
	return clusterStatuses, nodepoolStatuses
}

func testNodePool(specVersion string) *hyperfleetapi.NodePoolDetail {
	return &hyperfleetapi.NodePoolDetail{
		ID:         "np-test",
		ClusterID:  "cluster-test",
		Name:       "my-nodepool",
		Generation: 5,
		Spec: hyperfleetapi.NodePoolSpec{
			Release: hyperfleetapi.ReleaseSpec{Version: specVersion},
			Platform: hyperfleetapi.NodePoolGCPPlatform{
				Type: "GCP",
				GCP: hyperfleetapi.NodePoolGCPConf{
					ProjectID: "my-project",
					Region:    "us-central1",
					Zone:      "us-central1-b",
				},
			},
		},
	}
}

func testCluster() *hyperfleetapi.ClusterDetail {
	return &hyperfleetapi.ClusterDetail{
		ID:   "cluster-test",
		Name: "my-cluster",
		Spec: hyperfleetapi.ClusterSpec{
			Platform: hyperfleetapi.GCPPlatform{
				Type: "GCP",
				GCP: hyperfleetapi.GCPConfig{
					Subnet: "my-subnet",
					Region: "us-central1",
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Test cases
// ---------------------------------------------------------------------------

func TestReconcile_HappyPath(t *testing.T) {
	np := testNodePool("4.16.0")
	cluster := testCluster()
	clusterStatuses, nodepoolStatuses := buildReadyStatuses("4.16.0")

	var putCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/hyperfleet/v1/nodepools/"+np.ID:
			writeJSON(w, http.StatusOK, np)
		case r.Method == http.MethodGet && r.URL.Path == "/api/hyperfleet/v1/clusters/"+np.ClusterID:
			writeJSON(w, http.StatusOK, cluster)
		case r.Method == http.MethodGet && r.URL.Path == "/api/hyperfleet/v1/clusters/"+np.ClusterID+"/statuses":
			writeJSON(w, http.StatusOK, map[string]any{"items": clusterStatuses})
		case r.Method == http.MethodGet && r.URL.Path == "/api/hyperfleet/v1/nodepools/"+np.ID+"/statuses":
			writeJSON(w, http.StatusOK, map[string]any{"items": nodepoolStatuses})
		case r.Method == http.MethodPut && r.URL.Path == "/api/hyperfleet/v1/nodepools/"+np.ID+"/statuses":
			putCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Logf("unexpected: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	tr := mock.New()
	// Seed the mock with a status override so GetStatus returns something
	mwName := np.ID + "-" + adapterName
	tr.StatusOverrides["mc-us-c1/"+mwName] = &transport.ManifestWorkStatus{
		Conditions: []metav1.Condition{
			{Type: "Applied", Status: metav1.ConditionTrue, Reason: "AppliedSuccessfully"},
		},
		ResourceStatuses: []map[string]string{
			{
				"readyCondition":          "True",
				"allNodesHealthyCondition": "True",
				"replicas":                "2",
				"version":                 "4.16.0",
			},
		},
	}

	api := newTestAPIClient(t, srv)
	r := New(api, tr, newTestLogger(t))

	result, err := r.Reconcile(context.Background(), np.ID)
	require.NoError(t, err)
	require.Equal(t, requeueAfterApply, result.RequeueAfter)

	// Apply was called
	require.Len(t, tr.ApplyCalls, 1)
	require.Equal(t, "mc-us-c1", tr.ApplyCalls[0].TargetCluster)
	require.Equal(t, mwName, tr.ApplyCalls[0].Work.Name)

	// PUT was called
	require.True(t, putCalled)
}

func TestReconcile_NoPlacement(t *testing.T) {
	np := testNodePool("4.16.0")
	cluster := testCluster()

	// cluster statuses: no placement adapter
	clusterStatuses := hyperfleetapi.AdapterStatuses{
		{
			Adapter: "hc-adapter",
			Conditions: []hyperfleetapi.Condition{
				{Type: "Available", Status: condTrue},
			},
		},
	}
	nodepoolStatuses := hyperfleetapi.AdapterStatuses{
		{
			Adapter: "nodepool-vr-adapter",
			Data: map[string]any{
				"release_image":   "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
				"release_version": "4.16.0",
			},
		},
	}

	srv := buildTestServer(t, np, cluster, clusterStatuses, nodepoolStatuses)
	defer srv.Close()

	tr := mock.New()
	api := newTestAPIClient(t, srv)
	r := New(api, tr, newTestLogger(t))

	result, err := r.Reconcile(context.Background(), np.ID)
	require.NoError(t, err)
	require.Equal(t, requeueNotReady, result.RequeueAfter)
	require.Empty(t, tr.ApplyCalls)
}

func TestReconcile_HCNotAvailable(t *testing.T) {
	np := testNodePool("4.16.0")
	cluster := testCluster()

	clusterStatuses := hyperfleetapi.AdapterStatuses{
		{
			Adapter: "placement-adapter",
			Data: map[string]any{
				"managementClusterName": "mc-us-c1",
				"baseDomain":            "hc.example.com",
			},
		},
		{
			Adapter:    "hc-adapter",
			Conditions: []hyperfleetapi.Condition{}, // not Available
		},
	}
	nodepoolStatuses := hyperfleetapi.AdapterStatuses{
		{
			Adapter: "nodepool-vr-adapter",
			Data: map[string]any{
				"release_image":   "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
				"release_version": "4.16.0",
			},
		},
	}

	srv := buildTestServer(t, np, cluster, clusterStatuses, nodepoolStatuses)
	defer srv.Close()

	tr := mock.New()
	api := newTestAPIClient(t, srv)
	r := New(api, tr, newTestLogger(t))

	result, err := r.Reconcile(context.Background(), np.ID)
	require.NoError(t, err)
	require.Equal(t, requeueNotReady, result.RequeueAfter)
	require.Empty(t, tr.ApplyCalls)
}

func TestReconcile_NodePoolVRNotReady(t *testing.T) {
	np := testNodePool("4.16.0")
	cluster := testCluster()

	clusterStatuses, _ := buildReadyStatuses("4.16.0")
	// nodepool statuses: no nodepool-vr-adapter
	nodepoolStatuses := hyperfleetapi.AdapterStatuses{}

	srv := buildTestServer(t, np, cluster, clusterStatuses, nodepoolStatuses)
	defer srv.Close()

	tr := mock.New()
	api := newTestAPIClient(t, srv)
	r := New(api, tr, newTestLogger(t))

	result, err := r.Reconcile(context.Background(), np.ID)
	require.NoError(t, err)
	require.Equal(t, requeueNotReady, result.RequeueAfter)
	require.Empty(t, tr.ApplyCalls)
}

func TestReconcile_NodePoolNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/hyperfleet/v1/nodepools/np-missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		t.Logf("unexpected: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	tr := mock.New()
	api := newTestAPIClient(t, srv)
	r := New(api, tr, newTestLogger(t))

	result, err := r.Reconcile(context.Background(), "np-missing")
	require.NoError(t, err)
	require.Equal(t, time.Duration(0), result.RequeueAfter)
	require.Empty(t, tr.ApplyCalls)
}

func TestReconcile_VRVersionMismatch(t *testing.T) {
	// Spec says 4.16.0 but VR reports 4.15.0 → should requeue
	np := testNodePool("4.16.0")
	cluster := testCluster()

	clusterStatuses, _ := buildReadyStatuses("4.16.0")
	nodepoolStatuses := hyperfleetapi.AdapterStatuses{
		{
			Adapter: "nodepool-vr-adapter",
			Data: map[string]any{
				"release_image":   "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
				"release_version": "4.15.0", // mismatch
			},
		},
	}

	srv := buildTestServer(t, np, cluster, clusterStatuses, nodepoolStatuses)
	defer srv.Close()

	tr := mock.New()
	api := newTestAPIClient(t, srv)
	r := New(api, tr, newTestLogger(t))

	result, err := r.Reconcile(context.Background(), np.ID)
	require.NoError(t, err)
	require.Equal(t, requeueNotReady, result.RequeueAfter)
	require.Empty(t, tr.ApplyCalls)
}
