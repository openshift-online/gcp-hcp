package nodepoolvrresolution

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common/hyperfleetapi"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/adapters/versionresolution"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
)

// ---- helpers ----------------------------------------------------------------

func newTestLogger(t *testing.T) logger.Logger {
	t.Helper()
	log, err := logger.NewLogger(logger.Config{
		Level:     "error",
		Format:    logger.FormatText,
		Component: "test",
		Version:   "test",
	})
	require.NoError(t, err)
	return log
}

// mockHFServer builds an httptest.Server that responds to the HyperFleet API
// nodepool endpoints used by the reconciler.
type mockHFServer struct {
	nodepool         *hyperfleetapi.NodePoolDetail
	statuses         hyperfleetapi.AdapterStatuses
	putCalls         []hyperfleetapi.StatusPayload
	nodepoolNotFound bool
}

func (m *mockHFServer) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/hyperfleet/v1/nodepools/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		isStatuses := strings.HasSuffix(path, "/statuses")

		switch {
		case r.Method == http.MethodGet && !isStatuses:
			if m.nodepoolNotFound {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(m.nodepool) //nolint:errcheck

		case r.Method == http.MethodGet && isStatuses:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(m.statuses) //nolint:errcheck

		case r.Method == http.MethodPut && isStatuses:
			var payload hyperfleetapi.StatusPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			m.putCalls = append(m.putCalls, payload)
			w.WriteHeader(http.StatusOK)

		default:
			http.NotFound(w, r)
		}
	})

	return mux
}

// newMockCincinnati builds a simple httptest server that returns a Cincinnati
// graph containing the given release, or an empty graph if release is nil.
func newMockCincinnati(release *versionresolution.ReleaseInfo) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type graph struct {
			Nodes []versionresolution.ReleaseInfo `json:"nodes"`
		}
		g := graph{}
		if release != nil {
			g.Nodes = []versionresolution.ReleaseInfo{*release}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(g) //nolint:errcheck
	}))
}

// buildReconciler wires up a Reconciler from mock servers.
func buildReconciler(t *testing.T, mock *mockHFServer, cincSrv *httptest.Server) *Reconciler {
	t.Helper()
	hfSrv := httptest.NewServer(mock.handler())
	t.Cleanup(hfSrv.Close)

	hfClient := hyperfleetapi.New(hfSrv.URL, "v1", newTestLogger(t))
	cincClient := versionresolution.NewCincinnatiClient(cincSrv.URL, "amd64")
	return NewReconciler(hfClient, cincClient, newTestLogger(t))
}

// ---- tests ------------------------------------------------------------------

func TestReconciler_HappyPath(t *testing.T) {
	release := &versionresolution.ReleaseInfo{
		Version: "4.22.0-ec.4",
		Payload: "quay.io/openshift-release-dev/ocp-release:4.22.0-ec.4-x86_64",
	}
	cincSrv := newMockCincinnati(release)
	defer cincSrv.Close()

	mock := &mockHFServer{
		nodepool: &hyperfleetapi.NodePoolDetail{
			ID:         "np-1",
			ClusterID:  "cluster-1",
			Name:       "np-1",
			Generation: 3,
			Spec: hyperfleetapi.NodePoolSpec{
				Release: hyperfleetapi.ReleaseSpec{Version: "4.22.0-ec.4"},
			},
		},
		// No prior statuses → version not yet resolved.
		statuses: hyperfleetapi.AdapterStatuses{},
	}

	r := buildReconciler(t, mock, cincSrv)
	result, err := r.Reconcile(context.Background(), "np-1")

	require.NoError(t, err)
	require.Equal(t, common.Result{RequeueAfter: requeueLong}, result)
	require.Len(t, mock.putCalls, 1, "expected one PUT")

	put := mock.putCalls[0]
	require.Equal(t, adapterName, put.Adapter)
	require.Equal(t, int64(3), put.ObservedGeneration)
	require.Equal(t, release.Payload, put.Data["release_image"])
	require.Equal(t, "4.22.0-ec.4", put.Data["release_version"])
	require.Equal(t, "candidate-4.22", put.Data["release_channel"])
	require.Equal(t, "candidate", put.Data["release_channel_group"])
	require.Len(t, put.Conditions, 2)

	// Verify Applied condition.
	require.Equal(t, "Applied", put.Conditions[0].Type)
	require.Equal(t, "True", put.Conditions[0].Status)
	require.Equal(t, "VersionResolved", put.Conditions[0].Reason)

	// Verify Available condition.
	require.Equal(t, "Available", put.Conditions[1].Type)
	require.Equal(t, "True", put.Conditions[1].Status)
	require.Equal(t, "ReleaseImageAvailable", put.Conditions[1].Reason)

	// Verify ObservedTime is a valid RFC3339 timestamp.
	_, parseErr := time.Parse(time.RFC3339, put.ObservedTime)
	require.NoError(t, parseErr)
}

func TestReconciler_AlreadyResolved(t *testing.T) {
	cincSrv := newMockCincinnati(nil)
	defer cincSrv.Close()

	mock := &mockHFServer{
		nodepool: &hyperfleetapi.NodePoolDetail{
			ID:        "np-2",
			ClusterID: "cluster-1",
			Name:      "np-2",
			Spec: hyperfleetapi.NodePoolSpec{
				Release: hyperfleetapi.ReleaseSpec{Version: "4.22.0-ec.4"},
			},
		},
		statuses: hyperfleetapi.AdapterStatuses{
			{
				Adapter: adapterName,
				Data: map[string]any{
					"release_image":         "quay.io/openshift-release-dev/ocp-release:4.22.0-ec.4-x86_64",
					"release_version":       "4.22.0-ec.4",
					"release_channel":       "candidate-4.22",
					"release_channel_group": "candidate",
				},
			},
		},
	}

	r := buildReconciler(t, mock, cincSrv)
	result, err := r.Reconcile(context.Background(), "np-2")

	require.NoError(t, err)
	require.Equal(t, common.Result{RequeueAfter: requeueLong}, result)
	// No PUT should have been made.
	require.Empty(t, mock.putCalls)
}

func TestReconciler_NodepoolNotFound(t *testing.T) {
	cincSrv := newMockCincinnati(nil)
	defer cincSrv.Close()

	mock := &mockHFServer{nodepoolNotFound: true}

	r := buildReconciler(t, mock, cincSrv)
	result, err := r.Reconcile(context.Background(), "np-404")

	require.NoError(t, err)
	require.Equal(t, common.Result{}, result)
}

func TestReconciler_VersionNotInCincinnati(t *testing.T) {
	// Cincinnati returns an empty graph (no matching node).
	cincSrv := newMockCincinnati(nil)
	defer cincSrv.Close()

	mock := &mockHFServer{
		nodepool: &hyperfleetapi.NodePoolDetail{
			ID:        "np-5",
			ClusterID: "cluster-1",
			Name:      "np-5",
			Spec: hyperfleetapi.NodePoolSpec{
				Release: hyperfleetapi.ReleaseSpec{Version: "4.22.0-ec.4"},
			},
		},
		statuses: hyperfleetapi.AdapterStatuses{},
	}

	r := buildReconciler(t, mock, cincSrv)
	result, err := r.Reconcile(context.Background(), "np-5")

	require.NoError(t, err)
	require.Equal(t, common.Result{RequeueAfter: requeueShort}, result)
	require.Empty(t, mock.putCalls)
}

func TestReconciler_EmptyVersion(t *testing.T) {
	cincSrv := newMockCincinnati(nil)
	defer cincSrv.Close()

	mock := &mockHFServer{
		nodepool: &hyperfleetapi.NodePoolDetail{
			ID:        "np-6",
			ClusterID: "cluster-1",
			Name:      "np-6",
			Spec: hyperfleetapi.NodePoolSpec{
				// Release.Version is empty.
				Release: hyperfleetapi.ReleaseSpec{Version: ""},
			},
		},
		statuses: hyperfleetapi.AdapterStatuses{},
	}

	r := buildReconciler(t, mock, cincSrv)
	result, err := r.Reconcile(context.Background(), "np-6")

	require.NoError(t, err)
	require.Equal(t, common.Result{RequeueAfter: requeueShort}, result)
	require.Empty(t, mock.putCalls)
}

func TestBuildChannel(t *testing.T) {
	cases := []struct {
		version      string
		channelGroup string
		want         string
		wantErr      bool
	}{
		{"4.22.0-ec.4", "candidate", "candidate-4.22", false},
		{"4.16.3", "candidate", "candidate-4.16", false},
		{"4.15.0", "candidate", "candidate-4.15", false},
		{"4", "candidate", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			got, err := buildChannel(tc.version, tc.channelGroup)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}
