package gc

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"github.com/thetechnick/orlop/pkg/apiserver/storage/memory"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	gv := schema.GroupVersion{Group: "test.example.com", Version: "v1"}
	scheme.AddKnownTypeWithName(
		gv.WithKind("TestObject"),
		&unstructured.Unstructured{},
	)
	scheme.AddKnownTypeWithName(
		gv.WithKind("TestObjectList"),
		&unstructured.UnstructuredList{},
	)
	return scheme
}

func newTestMemoryStore(resourceType string) *memory.MemoryStore {
	scheme := newTestScheme()
	gvk := schema.GroupVersionKind{
		Group:   "test.example.com",
		Version: "v1",
		Kind:    "TestObject",
	}
	return memory.NewMemoryStore(resourceType, scheme, gvk)
}

func newObj(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "test.example.com/v1",
			"kind":       "TestObject",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

func newObjWithOwnerRef(name, namespace, ownerName, ownerKind, ownerUID string) *unstructured.Unstructured {
	obj := newObj(name, namespace)
	obj.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "test.example.com/v1",
			Kind:       ownerKind,
			Name:       ownerName,
			UID:        types.UID("uid-" + ownerUID),
		},
	})
	return obj
}

// runOneGCCycle starts the collector and waits long enough for one
// collectGarbage() call (which runs immediately on Start), then cancels.
func runOneGCCycle(stores map[string]storage.ResourceStore) {
	c := NewCollector(stores, time.Hour, logr.Discard())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Start(ctx)
		close(done)
	}()
	// Give the immediate collectGarbage() call time to run
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
}

func TestCollector_ObjectWithoutOwnerRefs_NotDeleted(t *testing.T) {
	ctx := context.Background()
	store := newTestMemoryStore("objects")
	obj := newObj("standalone", "default")
	if err := store.Create(ctx, obj); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	stores := map[string]storage.ResourceStore{
		"objects": store,
	}
	runOneGCCycle(stores)

	// Object should still exist
	_, err := store.Get(ctx, "default", "standalone")
	if err != nil {
		t.Errorf("object without owner refs was deleted: %v", err)
	}
}

func TestCollector_ObjectWithValidOwnerRef_NotDeleted(t *testing.T) {
	ctx := context.Background()
	store := newTestMemoryStore("objects")

	// Create the owner
	owner := newObj("my-owner", "default")
	if err := store.Create(ctx, owner); err != nil {
		t.Fatalf("Create owner failed: %v", err)
	}

	// Create a dependent that references the owner
	dependent := newObjWithOwnerRef("my-dependent", "default", "my-owner", "TestObject", "owner1")
	if err := store.Create(ctx, dependent); err != nil {
		t.Fatalf("Create dependent failed: %v", err)
	}

	stores := map[string]storage.ResourceStore{
		"objects": store,
	}
	runOneGCCycle(stores)

	// Dependent should still exist because its owner exists
	_, err := store.Get(ctx, "default", "my-dependent")
	if err != nil {
		t.Errorf("object with valid owner ref was deleted: %v", err)
	}
}

func TestCollector_ObjectWithMissingOwnerRef_Deleted(t *testing.T) {
	ctx := context.Background()
	store := newTestMemoryStore("objects")

	// Create a dependent whose owner does not exist
	dependent := newObjWithOwnerRef("orphan", "default", "nonexistent-owner", "TestObject", "missing")
	if err := store.Create(ctx, dependent); err != nil {
		t.Fatalf("Create dependent failed: %v", err)
	}

	stores := map[string]storage.ResourceStore{
		"objects": store,
	}
	runOneGCCycle(stores)

	// Orphan should be deleted because its owner does not exist
	_, err := store.Get(ctx, "default", "orphan")
	if err == nil {
		t.Error("orphaned object was not deleted by garbage collector")
	}
}

func TestCollector_MultipleStoresScanned(t *testing.T) {
	ctx := context.Background()
	storeA := newTestMemoryStore("kindA")
	storeB := newTestMemoryStore("kindB")

	// Create an owner in storeA
	owner := newObj("owner-a", "default")
	if err := storeA.Create(ctx, owner); err != nil {
		t.Fatalf("Create owner in storeA failed: %v", err)
	}

	// Create a dependent in storeB whose owner is in storeA
	dependentValid := newObjWithOwnerRef("dep-valid", "default", "owner-a", "KindA", "a1")
	if err := storeB.Create(ctx, dependentValid); err != nil {
		t.Fatalf("Create valid dependent failed: %v", err)
	}

	// Create an orphan in storeA (owner does not exist in any store)
	orphan := newObjWithOwnerRef("orphan-a", "default", "ghost", "KindX", "x1")
	if err := storeA.Create(ctx, orphan); err != nil {
		t.Fatalf("Create orphan failed: %v", err)
	}

	stores := map[string]storage.ResourceStore{
		"kindA": storeA,
		"kindB": storeB,
	}
	runOneGCCycle(stores)

	// The valid dependent should still exist (owner found in storeA)
	if _, err := storeB.Get(ctx, "default", "dep-valid"); err != nil {
		t.Errorf("valid dependent in storeB was incorrectly deleted: %v", err)
	}

	// The orphan should be deleted (owner not found in any store)
	if _, err := storeA.Get(ctx, "default", "orphan-a"); err == nil {
		t.Error("orphaned object in storeA was not deleted by garbage collector")
	}

	// The owner itself should still exist (no owner refs on it)
	if _, err := storeA.Get(ctx, "default", "owner-a"); err != nil {
		t.Errorf("owner object was incorrectly deleted: %v", err)
	}
}

func TestCollector_StopEndsCollector(t *testing.T) {
	store := newTestMemoryStore("objects")
	stores := map[string]storage.ResourceStore{
		"objects": store,
	}

	c := NewCollector(stores, time.Hour, logr.Discard())

	done := make(chan struct{})
	go func() {
		c.Start(context.Background())
		close(done)
	}()

	// Give Start time to begin
	time.Sleep(50 * time.Millisecond)
	c.Stop()

	select {
	case <-done:
		// Collector stopped successfully
	case <-time.After(2 * time.Second):
		t.Fatal("collector did not stop within timeout")
	}
}
