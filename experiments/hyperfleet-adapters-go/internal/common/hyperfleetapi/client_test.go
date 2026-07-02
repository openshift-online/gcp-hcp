package hyperfleetapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
)

// newTestClient builds an HTTPClient pointed at the given test server.
func newTestClient(t *testing.T, server *httptest.Server) *HTTPClient {
	t.Helper()
	log, err := logger.NewLogger(logger.Config{
		Level:     "debug",
		Format:    logger.FormatText,
		Output:    "stdout",
		Component: "test",
		Version:   "test",
	})
	require.NoError(t, err)
	c := New(server.URL, "v1", log)
	// Use the test server's client so TLS/redirect behaviour is handled correctly.
	c.httpClient = server.Client()
	return c
}

// writeJSON writes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v) //nolint:errcheck
	}
}

// --------------------------------------------------------------------------
// GetCluster
// --------------------------------------------------------------------------

func TestGetCluster_Success(t *testing.T) {
	want := ClusterDetail{
		ID:         "cluster-1",
		Name:       "my-cluster",
		Generation: 3,
		CreatedBy:  "user@example.com",
		Spec: ClusterSpec{
			InfraID:   "infra-abc",
			IssuerURL: "https://oidc.example.com",
			ClusterID: "550e8400-e29b-41d4-a716-446655440000",
			Release:   ReleaseSpec{Version: "4.16.0"},
			Platform: GCPPlatform{
				ProjectID:      "my-project",
				Region:         "us-central1",
				Network:        "my-vpc",
				Subnet:         "my-subnet",
				EndpointAccess: "Private",
				WorkloadIdentity: WIFConfig{
					ProjectNumber: "123456789",
					PoolID:        "pool-id",
					ProviderID:    "provider-id",
					ServiceAccountsRef: WIFServiceAccounts{
						NodePool:        "np@project.iam.gserviceaccount.com",
						ControlPlane:    "cp@project.iam.gserviceaccount.com",
						CloudController: "cc@project.iam.gserviceaccount.com",
						Storage:         "st@project.iam.gserviceaccount.com",
						ImageRegistry:   "ir@project.iam.gserviceaccount.com",
						Network:         "net@project.iam.gserviceaccount.com",
					},
				},
			},
		},
		Status: ClusterStatus{
			Conditions: []Condition{
				{Type: "Reconciled", Status: "False"},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/hyperfleet/v1/clusters/cluster-1", r.URL.Path)
		writeJSON(w, http.StatusOK, want)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.GetCluster(context.Background(), "cluster-1")

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, want.Name, got.Name)
	assert.Equal(t, want.Generation, got.Generation)
	assert.Equal(t, want.CreatedBy, got.CreatedBy)
	assert.Equal(t, want.Spec.InfraID, got.Spec.InfraID)
	assert.Equal(t, want.Spec.Platform.ProjectID, got.Spec.Platform.ProjectID)
	assert.Equal(t, want.Spec.Platform.WorkloadIdentity.ServiceAccountsRef.NodePool,
		got.Spec.Platform.WorkloadIdentity.ServiceAccountsRef.NodePool)
	assert.Len(t, got.Status.Conditions, 1)
}

func TestGetCluster_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.GetCluster(context.Background(), "missing")

	require.Error(t, err)
	var nfe *NotFoundError
	require.True(t, errors.As(err, &nfe), "expected NotFoundError, got %T: %v", err, err)
}

func TestGetCluster_ServerError_RetriesAndFails(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	// Override backoff to keep tests fast — zero duration
	_, err := c.GetCluster(context.Background(), "cluster-1")

	require.Error(t, err)
	assert.EqualValues(t, maxRetryAttempts, callCount.Load(),
		"expected exactly %d attempts", maxRetryAttempts)
}

// --------------------------------------------------------------------------
// PutClusterStatus
// --------------------------------------------------------------------------

func TestPutClusterStatus_Success(t *testing.T) {
	var received StatusPayload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/hyperfleet/v1/clusters/cluster-1/statuses", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	payload := StatusPayload{
		Adapter:            "hc-adapter",
		ObservedGeneration: 5,
		ObservedTime:       "2026-07-01T00:00:00Z",
		Conditions: []Condition{
			{Type: "Applied", Status: "True", Reason: "Applied"},
		},
		Data: map[string]any{"key": "value"},
	}

	err := c.PutClusterStatus(context.Background(), "cluster-1", payload)
	require.NoError(t, err)
	assert.Equal(t, payload.Adapter, received.Adapter)
	assert.Equal(t, payload.ObservedGeneration, received.ObservedGeneration)
}

// --------------------------------------------------------------------------
// GetClusterStatuses and accessor methods
// --------------------------------------------------------------------------

func TestGetClusterStatuses_AccessorMethods(t *testing.T) {
	statuses := AdapterStatuses{
		{
			Adapter: "placement-adapter",
			Data: map[string]any{
				"managementClusterName": "mc-us-c1",
				"baseDomain":            "hc-us-central1-abc.example.com",
			},
		},
		{
			Adapter: "version-resolution-adapter",
			Data: map[string]any{
				"release_image":         "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
				"release_version":       "4.16.0",
				"release_channel":       "stable-4.16",
				"release_channel_group": "stable",
			},
		},
		{
			Adapter: "nodepool-vr-adapter",
			Data: map[string]any{
				"release_image":         "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
				"release_version":       "4.16.0",
				"release_channel":       "stable-4.16",
				"release_channel_group": "stable",
			},
		},
		{
			Adapter: "hc-adapter",
			Conditions: []Condition{
				{Type: "Available", Status: "True", Reason: "HostedClusterAvailable"},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/hyperfleet/v1/clusters/cluster-1/statuses", r.URL.Path)
		writeJSON(w, http.StatusOK, statuses)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.GetClusterStatuses(context.Background(), "cluster-1")
	require.NoError(t, err)
	require.Len(t, got, 4)

	// Placement accessor
	p := got.Placement()
	require.NotNil(t, p)
	assert.Equal(t, "mc-us-c1", p.ManagementClusterName)
	assert.Equal(t, "hc-us-central1-abc.example.com", p.BaseDomain)
	assert.True(t, p.Ready())

	// VR accessor
	vr := got.VersionResolution()
	require.NotNil(t, vr)
	assert.Equal(t, "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64", vr.ReleaseImage)
	assert.Equal(t, "4.16.0", vr.ReleaseVersion)
	assert.Equal(t, "stable-4.16", vr.ReleaseChannel)
	assert.True(t, vr.Ready())

	// NodePoolVR accessor
	npvr := got.NodePoolVR()
	require.NotNil(t, npvr)
	assert.Equal(t, "4.16.0", npvr.ReleaseVersion)
	assert.True(t, npvr.Ready())

	// HCAdapter accessor
	hc := got.HCAdapter()
	require.NotNil(t, hc)
	assert.True(t, hc.Available())
}

func TestGetClusterStatuses_MissingAdapters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, AdapterStatuses{})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.GetClusterStatuses(context.Background(), "cluster-1")
	require.NoError(t, err)

	assert.Nil(t, got.Placement())
	assert.Nil(t, got.VersionResolution())
	assert.Nil(t, got.NodePoolVR())
	assert.Nil(t, got.HCAdapter())

	// Ready/Available on nil pointers should return false, not panic.
	var p *PlacementData
	assert.False(t, p.Ready())
	var vr *VRData
	assert.False(t, vr.Ready())
	var npvr *NodePoolVRData
	assert.False(t, npvr.Ready())
	var hc *HCData
	assert.False(t, hc.Available())
}
