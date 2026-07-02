package placement

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/hyperfleetapi"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
)

// mockSelector is a simple Selector implementation for tests.
type mockSelector struct {
	mcName     string
	baseDomain string
	err        error
}

func (m *mockSelector) Select(_ context.Context, _ []Candidate) (string, string, error) {
	return m.mcName, m.baseDomain, m.err
}

// testLogger creates a logger for tests.
func testLogger(t *testing.T) logger.Logger {
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

// writeJSON writes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v) //nolint:errcheck
	}
}

// testCandidates returns a default candidate list for tests.
func testCandidates() []Candidate {
	return []Candidate{
		{Name: "mc-us-c1", BaseDomains: []string{"hc-us-central1-abc.example.com"}},
	}
}

// testCluster returns a ClusterDetail with optional conditions.
func testCluster(id string, conditions []hyperfleetapi.Condition) hyperfleetapi.ClusterDetail {
	return hyperfleetapi.ClusterDetail{
		ID:         id,
		Name:       "test-cluster",
		Generation: 1,
		Status: hyperfleetapi.ClusterStatus{
			Conditions: conditions,
		},
	}
}

// mockHyperfleetClient is a mock of hyperfleetapi.Client backed by an httptest.Server.
// It uses the standard net/http client configured to talk to the test server.
type mockHyperfleetClient struct {
	srv    *httptest.Server
	client *http.Client
}

func newMockClient(srv *httptest.Server) *mockHyperfleetClient {
	return &mockHyperfleetClient{
		srv:    srv,
		client: srv.Client(),
	}
}

func (m *mockHyperfleetClient) doGet(ctx context.Context, path string, dest interface{}) error {
	url := m.srv.URL + "/api/hyperfleet/v1" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode == http.StatusNotFound {
		return &hyperfleetapi.NotFoundError{Resource: path}
	}
	if dest != nil {
		return json.NewDecoder(resp.Body).Decode(dest)
	}
	return nil
}

func (m *mockHyperfleetClient) doPut(ctx context.Context, path string, body interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := m.srv.URL + "/api/hyperfleet/v1" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close() //nolint:errcheck
	return nil
}

func (m *mockHyperfleetClient) GetCluster(ctx context.Context, clusterID string) (*hyperfleetapi.ClusterDetail, error) {
	var c hyperfleetapi.ClusterDetail
	if err := m.doGet(ctx, fmt.Sprintf("/clusters/%s", clusterID), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (m *mockHyperfleetClient) GetClusterStatuses(ctx context.Context, clusterID string) (hyperfleetapi.AdapterStatuses, error) {
	var s hyperfleetapi.AdapterStatuses
	if err := m.doGet(ctx, fmt.Sprintf("/clusters/%s/statuses", clusterID), &s); err != nil {
		return nil, err
	}
	return s, nil
}

func (m *mockHyperfleetClient) PutClusterStatus(ctx context.Context, clusterID string, payload hyperfleetapi.StatusPayload) error {
	return m.doPut(ctx, fmt.Sprintf("/clusters/%s/statuses", clusterID), payload)
}

func (m *mockHyperfleetClient) GetNodePool(ctx context.Context, nodepoolID string) (*hyperfleetapi.NodePoolDetail, error) {
	var np hyperfleetapi.NodePoolDetail
	if err := m.doGet(ctx, fmt.Sprintf("/nodepools/%s", nodepoolID), &np); err != nil {
		return nil, err
	}
	return &np, nil
}

func (m *mockHyperfleetClient) GetNodePoolStatuses(ctx context.Context, nodepoolID string) (hyperfleetapi.AdapterStatuses, error) {
	var s hyperfleetapi.AdapterStatuses
	if err := m.doGet(ctx, fmt.Sprintf("/nodepools/%s/statuses", nodepoolID), &s); err != nil {
		return nil, err
	}
	return s, nil
}

func (m *mockHyperfleetClient) PutNodePoolStatus(ctx context.Context, nodepoolID string, payload hyperfleetapi.StatusPayload) error {
	return m.doPut(ctx, fmt.Sprintf("/nodepools/%s/statuses", nodepoolID), payload)
}

// Ensure mockHyperfleetClient implements Client.
var _ hyperfleetapi.Client = (*mockHyperfleetClient)(nil)

func TestReconciler(t *testing.T) {
	tests := []struct {
		name           string
		clusterID      string
		cluster        *hyperfleetapi.ClusterDetail // nil → 404
		statuses       hyperfleetapi.AdapterStatuses
		selector       *mockSelector
		expectPUT      bool
		expectedResult common.Result
		expectError    bool
	}{
		{
			name:      "happy path: selects MC and domain, PUTs status",
			clusterID: "cluster-1",
			cluster: func() *hyperfleetapi.ClusterDetail {
				c := testCluster("cluster-1", []hyperfleetapi.Condition{
					{Type: "Reconciled", Status: "False"},
				})
				return &c
			}(),
			statuses:       hyperfleetapi.AdapterStatuses{},
			selector:       &mockSelector{mcName: "mc-us-c1", baseDomain: "hc-us-central1-abc.example.com"},
			expectPUT:      true,
			expectedResult: common.Result{RequeueAfter: requeueAfter},
		},
		{
			name:      "already placed: no PUT, requeue",
			clusterID: "cluster-2",
			cluster: func() *hyperfleetapi.ClusterDetail {
				c := testCluster("cluster-2", nil)
				return &c
			}(),
			statuses: hyperfleetapi.AdapterStatuses{
				{
					Adapter: "placement-adapter",
					Data: map[string]any{
						"managementClusterName": "mc-us-c1",
						"baseDomain":            "hc-us-central1-abc.example.com",
					},
				},
			},
			selector:       &mockSelector{mcName: "mc-us-c1", baseDomain: "hc-us-central1-abc.example.com"},
			expectPUT:      false,
			expectedResult: common.Result{RequeueAfter: requeueAfter},
		},
		{
			name:           "cluster not found: return empty result, no error",
			clusterID:      "cluster-missing",
			cluster:        nil, // will return 404
			statuses:       hyperfleetapi.AdapterStatuses{},
			selector:       &mockSelector{mcName: "mc-us-c1", baseDomain: "hc-us-central1-abc.example.com"},
			expectPUT:      false,
			expectedResult: common.Result{},
			expectError:    false,
		},
		{
			name:      "reconciled cluster: skip, requeue",
			clusterID: "cluster-3",
			cluster: func() *hyperfleetapi.ClusterDetail {
				c := testCluster("cluster-3", []hyperfleetapi.Condition{
					{Type: "Reconciled", Status: "True", Reason: "ReconcileComplete"},
				})
				return &c
			}(),
			statuses:       hyperfleetapi.AdapterStatuses{},
			selector:       &mockSelector{mcName: "mc-us-c1", baseDomain: "hc-us-central1-abc.example.com"},
			expectPUT:      false,
			expectedResult: common.Result{RequeueAfter: requeueAfter},
		},
		{
			name:      "selector error: return error",
			clusterID: "cluster-4",
			cluster: func() *hyperfleetapi.ClusterDetail {
				c := testCluster("cluster-4", nil)
				return &c
			}(),
			statuses:       hyperfleetapi.AdapterStatuses{},
			selector:       &mockSelector{err: fmt.Errorf("no candidates available")},
			expectPUT:      false,
			expectedResult: common.Result{},
			expectError:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			putCalled := false
			var captured hyperfleetapi.StatusPayload

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				clusterPath := fmt.Sprintf("/api/hyperfleet/v1/clusters/%s", tc.clusterID)
				statusesPath := fmt.Sprintf("/api/hyperfleet/v1/clusters/%s/statuses", tc.clusterID)

				switch {
				case r.Method == http.MethodGet && r.URL.Path == clusterPath:
					if tc.cluster == nil {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					writeJSON(w, http.StatusOK, tc.cluster)

				case r.Method == http.MethodGet && r.URL.Path == statusesPath:
					writeJSON(w, http.StatusOK, tc.statuses)

				case r.Method == http.MethodPut && r.URL.Path == statusesPath:
					putCalled = true
					require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
					w.WriteHeader(http.StatusOK)

				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer srv.Close()

			hfClient := newMockClient(srv)
			reconciler := NewReconciler(hfClient, tc.selector, testCandidates(), testLogger(t))
			result, err := reconciler.Reconcile(context.Background(), tc.clusterID)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectedResult, result)
			require.Equal(t, tc.expectPUT, putCalled, "PUT called mismatch")

			if tc.expectPUT {
				require.Equal(t, adapterName, captured.Adapter)
				require.Equal(t, tc.selector.mcName, captured.Data["managementClusterName"])
				require.Equal(t, tc.selector.baseDomain, captured.Data["baseDomain"])
				require.Len(t, captured.Conditions, 2)
				require.Equal(t, "Applied", captured.Conditions[0].Type)
				require.Equal(t, "True", captured.Conditions[0].Status)
				require.Equal(t, "Available", captured.Conditions[1].Type)
				require.Equal(t, "True", captured.Conditions[1].Status)
				// ObservedTime must be a valid RFC3339 timestamp.
				_, timeErr := time.Parse(time.RFC3339, captured.ObservedTime)
				require.NoError(t, timeErr)
			}
		})
	}
}

func TestRoundRobinSelector(t *testing.T) {
	t.Run("selects candidates in round-robin order", func(t *testing.T) {
		candidates := []Candidate{
			{Name: "mc-a", BaseDomains: []string{"a.example.com"}},
			{Name: "mc-b", BaseDomains: []string{"b.example.com"}},
		}

		sel := NewRoundRobinSelector()
		ctx := context.Background()

		mc1, domain1, err := sel.Select(ctx, candidates)
		require.NoError(t, err)
		require.Equal(t, "mc-a", mc1)
		require.Equal(t, "a.example.com", domain1)

		mc2, domain2, err := sel.Select(ctx, candidates)
		require.NoError(t, err)
		require.Equal(t, "mc-b", mc2)
		require.Equal(t, "b.example.com", domain2)

		// Wraps around.
		mc3, _, err := sel.Select(ctx, candidates)
		require.NoError(t, err)
		require.Equal(t, "mc-a", mc3)
	})

	t.Run("returns error when no candidates", func(t *testing.T) {
		sel := NewRoundRobinSelector()
		_, _, err := sel.Select(context.Background(), nil)
		require.Error(t, err)
	})

	t.Run("returns error when candidate has no base domains", func(t *testing.T) {
		sel := NewRoundRobinSelector()
		candidates := []Candidate{
			{Name: "mc-empty", BaseDomains: []string{}},
		}
		_, _, err := sel.Select(context.Background(), candidates)
		require.Error(t, err)
	})
}
