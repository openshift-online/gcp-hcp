package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/transport"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	workv1 "open-cluster-management.io/api/work/v1"
)

// ApplyCall records arguments from an Apply call.
type ApplyCall struct {
	TargetCluster string
	Work          *workv1.ManifestWork
}

// DeleteCall records arguments from a Delete call.
type DeleteCall struct {
	TargetCluster string
	Name          string
}

// Client is an in-memory implementation of transport.Client for use in tests.
type Client struct {
	mu sync.RWMutex

	// store holds ManifestWorks keyed by "targetCluster/name".
	store map[string]*workv1.ManifestWork

	// StatusOverrides allows tests to inject a specific status for GetStatus calls.
	// Key format: "targetCluster/name".
	StatusOverrides map[string]*transport.ManifestWorkStatus

	// ApplyCalls records all Apply invocations for test assertions.
	ApplyCalls []ApplyCall

	// DeleteCalls records all Delete invocations for test assertions.
	DeleteCalls []DeleteCall
}

// Ensure Client implements transport.Client.
var _ transport.Client = (*Client)(nil)

// New creates a new in-memory mock Client.
func New() *Client {
	return &Client{
		store:           make(map[string]*workv1.ManifestWork),
		StatusOverrides: make(map[string]*transport.ManifestWorkStatus),
	}
}

func storeKey(targetCluster, name string) string {
	return targetCluster + "/" + name
}

// Apply stores the ManifestWork in memory and records the call.
func (c *Client) Apply(ctx context.Context, targetCluster string, mw *workv1.ManifestWork) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ApplyCalls = append(c.ApplyCalls, ApplyCall{
		TargetCluster: targetCluster,
		Work:          mw,
	})

	key := storeKey(targetCluster, mw.GetName())
	c.store[key] = mw.DeepCopy()
	return nil
}

// GetStatus returns the ManifestWork status. If a StatusOverride is set for this key
// it is returned; otherwise an empty status is returned. Returns not-found if the
// ManifestWork has never been applied.
func (c *Client) GetStatus(ctx context.Context, targetCluster, name string) (*transport.ManifestWorkStatus, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := storeKey(targetCluster, name)

	if _, ok := c.store[key]; !ok {
		gr := schema.GroupResource{Group: "work.open-cluster-management.io", Resource: "manifestworks"}
		return nil, apierrors.NewNotFound(gr, fmt.Sprintf("%s/%s", targetCluster, name))
	}

	if override, ok := c.StatusOverrides[key]; ok {
		return override, nil
	}

	return &transport.ManifestWorkStatus{}, nil
}

// Delete removes the ManifestWork from the in-memory store and records the call.
// Not-found is silently ignored.
func (c *Client) Delete(ctx context.Context, targetCluster, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.DeleteCalls = append(c.DeleteCalls, DeleteCall{
		TargetCluster: targetCluster,
		Name:          name,
	})

	key := storeKey(targetCluster, name)
	delete(c.store, key)
	return nil
}

// Get returns the stored ManifestWork or nil if not found. Useful for test assertions.
func (c *Client) Get(targetCluster, name string) *workv1.ManifestWork {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := storeKey(targetCluster, name)
	mw, ok := c.store[key]
	if !ok {
		return nil
	}
	return mw.DeepCopy()
}

// Reset clears all stored state and recorded calls. Useful between test cases.
func (c *Client) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store = make(map[string]*workv1.ManifestWork)
	c.StatusOverrides = make(map[string]*transport.ManifestWorkStatus)
	c.ApplyCalls = nil
	c.DeleteCalls = nil
}
