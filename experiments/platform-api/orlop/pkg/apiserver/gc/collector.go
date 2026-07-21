package gc

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Collector implements garbage collection for objects with owner references.
// It periodically scans all objects and deletes those whose owners no longer exist.
type Collector struct {
	stores   map[string]storage.ResourceStore
	interval time.Duration
	logger   logr.Logger
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewCollector creates a new garbage collector.
func NewCollector(stores map[string]storage.ResourceStore, interval time.Duration, logger logr.Logger) *Collector {
	if logger.GetSink() == nil {
		logger = logr.Discard()
	}
	return &Collector{
		stores:   stores,
		interval: interval,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the garbage collection loop in a background goroutine.
func (c *Collector) Start(ctx context.Context) {
	c.logger.Info("Starting garbage collector", "interval", c.interval)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Run once immediately
	c.collectGarbage()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Garbage collector stopped")
			return
		case <-c.stopCh:
			c.logger.Info("Garbage collector stopped")
			return
		case <-ticker.C:
			c.collectGarbage()
		}
	}
}

// Stop gracefully stops the garbage collector.
func (c *Collector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

// collectGarbage performs one garbage collection cycle across all stores.
func (c *Collector) collectGarbage() {
	c.logger.V(1).Info("Running garbage collection cycle")
	startTime := time.Now()

	deleted := 0
	checked := 0

	ctx := context.Background()

	for resourceType, store := range c.stores {
		// List all objects in this store
		list, err := store.List(ctx, storage.ListOptions{})
		if err != nil {
			c.logger.Error(err, "Failed to list objects for GC", "resourceType", resourceType)
			continue
		}

		items, err := meta.ExtractList(list)
		if err != nil {
			c.logger.Error(err, "Failed to extract list items", "resourceType", resourceType)
			continue
		}

		// Check each object for orphaned owner references
		for _, item := range items {
			checked++
			
			obj, ok := item.(client.Object)
			if !ok {
				continue
			}

			accessor, err := meta.Accessor(obj)
			if err != nil {
				continue
			}

			ownerRefs := accessor.GetOwnerReferences()
			if len(ownerRefs) == 0 {
				continue
			}

			// Check if any owner still exists
			shouldDelete := false
			for _, ownerRef := range ownerRefs {
				exists, err := c.ownerExists(ownerRef, accessor.GetNamespace())
				if err != nil {
					c.logger.Error(err, "Failed to check owner existence", 
						"object", accessor.GetName(), 
						"namespace", accessor.GetNamespace(),
						"owner", ownerRef.Name)
					continue
				}

				if !exists {
					c.logger.Info("Owner no longer exists, deleting dependent",
						"object", accessor.GetName(),
						"namespace", accessor.GetNamespace(),
						"owner", ownerRef.Name,
						"ownerKind", ownerRef.Kind)
					shouldDelete = true
					break
				}
			}

			if shouldDelete {
				// Delete the object
				if err := store.Delete(ctx, accessor.GetNamespace(), accessor.GetName()); err != nil {
					if !errors.IsNotFound(err) {
						c.logger.Error(err, "Failed to delete orphaned object",
							"object", accessor.GetName(),
							"namespace", accessor.GetNamespace())
					}
				} else {
					deleted++
					c.logger.V(1).Info("Deleted orphaned object",
						"object", accessor.GetName(),
						"namespace", accessor.GetNamespace())
				}
			}
		}
	}

	duration := time.Since(startTime)
	c.logger.Info("Garbage collection cycle complete",
		"duration", duration,
		"checked", checked,
		"deleted", deleted)
}

// ownerExists checks if an owner object still exists in storage.
func (c *Collector) ownerExists(ownerRef metav1.OwnerReference, namespace string) (bool, error) {
	ctx := context.Background()
	// Find the store for the owner's resource type
	// This is a simplified check - in a real implementation, we'd need to map
	// Kind to resource type more accurately
	for _, store := range c.stores {
		_, err := store.Get(ctx, namespace, ownerRef.Name)
		if err == nil {
			// Owner exists
			return true, nil
		}
		if errors.IsNotFound(err) {
			// Owner doesn't exist in this store, continue checking others
			continue
		}
		// Some other error occurred
		return false, err
	}

	// Owner not found in any store
	return false, nil
}
