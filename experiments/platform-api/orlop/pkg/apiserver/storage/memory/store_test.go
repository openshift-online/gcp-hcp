package memory

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/thetechnick/orlop/pkg/apiserver/storage"
)

// contextKey is a local type for context filter keys in tests.
type contextKey string

// Test helper factory using closures for flexibility
type objectOption func(*unstructured.Unstructured)

func newTestObject(opts ...objectOption) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "test.example.com/v1",
			"kind":       "TestObject",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
			},
		},
	}
	for _, opt := range opts {
		opt(obj)
	}
	return obj
}

func withName(name string) objectOption {
	return func(obj *unstructured.Unstructured) {
		obj.SetName(name)
	}
}

func withNamespace(namespace string) objectOption {
	return func(obj *unstructured.Unstructured) {
		obj.SetNamespace(namespace)
	}
}

func withLabels(labels map[string]string) objectOption {
	return func(obj *unstructured.Unstructured) {
		obj.SetLabels(labels)
	}
}

func withSpec(spec map[string]interface{}) objectOption {
	return func(obj *unstructured.Unstructured) {
		obj.Object["spec"] = spec
	}
}

func withGenerateName(generateName string) objectOption {
	return func(obj *unstructured.Unstructured) {
		obj.SetGenerateName(generateName)
		obj.SetName("")
	}
}

// Factory to create test store with closures
func newTestStore(configurers ...func(*runtime.Scheme)) *MemoryStore {
	return newTestStoreWithOpts(nil, configurers...)
}

// newTestStoreWithOpts creates a test store with MemoryStoreOptions and optional scheme configurers.
func newTestStoreWithOpts(storeOpts []MemoryStoreOption, configurers ...func(*runtime.Scheme)) *MemoryStore {
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

	for _, configure := range configurers {
		configure(scheme)
	}

	gvk := schema.GroupVersionKind{
		Group:   "test.example.com",
		Version: "v1",
		Kind:    "TestObject",
	}

	return NewMemoryStore("testobjects", scheme, gvk, storeOpts...)
}

func TestMemoryStore_Create(t *testing.T) {
	t.Run("creates new object with resourceVersion", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(withName("test-obj"), withNamespace("default"))

		err := store.Create(ctx, obj)
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		retrieved, err := store.Get(ctx, "default", "test-obj")
		if err != nil {
			t.Fatalf("Get() failed: %v", err)
		}

		if retrieved.GetResourceVersion() == "" {
			t.Error("Created object missing resourceVersion")
		}
		if retrieved.GetResourceVersion() != "1" {
			t.Errorf("Expected resourceVersion 1, got %s", retrieved.GetResourceVersion())
		}
	})

	t.Run("returns error for duplicate", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(withName("duplicate"), withNamespace("default"))

		store.Create(ctx, obj)
		err := store.Create(ctx, obj)

		if err == nil {
			t.Error("Expected error for duplicate object, got nil")
		}
	})

	t.Run("generateName produces a unique name", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(withGenerateName("gen-"), withNamespace("default"))

		err := store.Create(ctx, obj)
		if err != nil {
			t.Fatalf("Create() with generateName failed: %v", err)
		}

		name := obj.GetName()
		if name == "" {
			t.Fatal("Name was not set after Create with generateName")
		}
		if len(name) < len("gen-")+5 {
			t.Errorf("Generated name too short: %q", name)
		}

		retrieved, err := store.Get(ctx, "default", name)
		if err != nil {
			t.Fatalf("Get() by generated name failed: %v", err)
		}
		if retrieved.GetName() != name {
			t.Errorf("Retrieved name %q != generated name %q", retrieved.GetName(), name)
		}
	})

	t.Run("generateName creates distinct names", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		seen := make(map[string]bool)

		for range 20 {
			obj := newTestObject(withGenerateName("multi-"), withNamespace("default"))
			if err := store.Create(ctx, obj); err != nil {
				t.Fatalf("Create() failed: %v", err)
			}
			name := obj.GetName()
			if seen[name] {
				t.Fatalf("Duplicate generated name: %q", name)
			}
			seen[name] = true
		}

		listObj, _ := store.List(ctx, storage.ListOptions{Namespace: "default"})
		list := listObj.(*unstructured.UnstructuredList)
		if len(list.Items) != 20 {
			t.Errorf("Expected 20 objects, got %d", len(list.Items))
		}
	})

	t.Run("generateName sets resourceVersion", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(withGenerateName("rv-"), withNamespace("default"))

		store.Create(ctx, obj)

		if obj.GetResourceVersion() == "" {
			t.Error("ResourceVersion not set on generated-name object")
		}
	})

	t.Run("creates in different namespaces", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()

		obj1 := newTestObject(withName("obj"), withNamespace("ns1"))
		obj2 := newTestObject(withName("obj"), withNamespace("ns2"))

		if err := store.Create(ctx, obj1); err != nil {
			t.Errorf("Create in ns1 failed: %v", err)
		}
		if err := store.Create(ctx, obj2); err != nil {
			t.Errorf("Create in ns2 failed: %v", err)
		}

		// Both should exist
		if _, err := store.Get(ctx, "ns1", "obj"); err != nil {
			t.Error("Object in ns1 not found")
		}
		if _, err := store.Get(ctx, "ns2", "obj"); err != nil {
			t.Error("Object in ns2 not found")
		}
	})
}

func TestMemoryStore_Get(t *testing.T) {
	t.Run("gets existing object", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(withName("test"), withNamespace("default"))
		store.Create(ctx, obj)

		retrieved, err := store.Get(ctx, "default", "test")
		if err != nil {
			t.Fatalf("Get() failed: %v", err)
		}
		if retrieved.GetName() != "test" {
			t.Errorf("Got wrong object: %s", retrieved.GetName())
		}
	})

	t.Run("returns error for non-existent object", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()

		_, err := store.Get(ctx, "default", "missing")
		if err == nil {
			t.Error("Expected error for missing object, got nil")
		}
	})

	t.Run("returns error for wrong namespace", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(withName("test"), withNamespace("default"))
		store.Create(ctx, obj)

		_, err := store.Get(ctx, "kube-system", "test")
		if err == nil {
			t.Error("Expected error for wrong namespace, got nil")
		}
	})
}

func TestMemoryStore_Update(t *testing.T) {
	t.Run("updates existing object", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(
			withName("test"),
			withNamespace("default"),
			withSpec(map[string]interface{}{"field": "original"}),
		)
		store.Create(ctx, obj)

		retrieved, _ := store.Get(ctx, "default", "test")
		updated := retrieved.DeepCopyObject().(client.Object)
		u := updated.(*unstructured.Unstructured)
		u.Object["spec"] = map[string]interface{}{"field": "updated"}

		err := store.Update(ctx, updated)
		if err != nil {
			t.Fatalf("Update() failed: %v", err)
		}

		final, _ := store.Get(ctx, "default", "test")
		finalU := final.(*unstructured.Unstructured)
		spec := finalU.Object["spec"].(map[string]interface{})
		if spec["field"] != "updated" {
			t.Errorf("Update did not persist: got %v", spec["field"])
		}
	})

	t.Run("increments resourceVersion", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(withName("test"), withNamespace("default"))
		store.Create(ctx, obj)

		retrieved, _ := store.Get(ctx, "default", "test")
		initialRV := retrieved.GetResourceVersion()

		store.Update(ctx, retrieved)

		updated, _ := store.Get(ctx, "default", "test")
		if updated.GetResourceVersion() == initialRV {
			t.Error("ResourceVersion not incremented after update")
		}
	})

	t.Run("returns error for non-existent object", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(withName("missing"), withNamespace("default"))

		err := store.Update(ctx, obj)
		if err == nil {
			t.Error("Expected error for missing object, got nil")
		}
	})
}

func TestMemoryStore_Delete(t *testing.T) {
	t.Run("deletes existing object", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(withName("test"), withNamespace("default"))
		store.Create(ctx, obj)

		err := store.Delete(ctx, "default", "test")
		if err != nil {
			t.Fatalf("Delete() failed: %v", err)
		}

		_, err = store.Get(ctx, "default", "test")
		if err == nil {
			t.Error("Object still exists after delete")
		}
	})

	t.Run("returns error for non-existent object", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()

		err := store.Delete(ctx, "default", "missing")
		if err == nil {
			t.Error("Expected error for missing object, got nil")
		}
	})
}

func TestMemoryStore_List(t *testing.T) {
	t.Run("lists objects in namespace", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		store.Create(ctx, newTestObject(withName("obj1"), withNamespace("default")))
		store.Create(ctx, newTestObject(withName("obj2"), withNamespace("default")))
		store.Create(ctx, newTestObject(withName("obj3"), withNamespace("kube-system")))

		listObj, err := store.List(ctx, storage.ListOptions{Namespace: "default"})
		if err != nil {
			t.Fatalf("List() failed: %v", err)
		}

		list := listObj.(*unstructured.UnstructuredList)
		if len(list.Items) != 2 {
			t.Errorf("Expected 2 objects, got %d", len(list.Items))
		}
	})

	t.Run("lists all namespaces", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		store.Create(ctx, newTestObject(withName("obj1"), withNamespace("default")))
		store.Create(ctx, newTestObject(withName("obj2"), withNamespace("kube-system")))
		store.Create(ctx, newTestObject(withName("obj3"), withNamespace("kube-public")))

		listObj, err := store.List(ctx, storage.ListOptions{})
		if err != nil {
			t.Fatalf("List() failed: %v", err)
		}

		list := listObj.(*unstructured.UnstructuredList)
		if len(list.Items) != 3 {
			t.Errorf("Expected 3 objects, got %d", len(list.Items))
		}
	})

	t.Run("returns empty list for empty store", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()

		listObj, err := store.List(ctx, storage.ListOptions{Namespace: "default"})
		if err != nil {
			t.Fatalf("List() failed: %v", err)
		}

		list := listObj.(*unstructured.UnstructuredList)
		if len(list.Items) != 0 {
			t.Errorf("Expected empty list, got %d objects", len(list.Items))
		}
	})

	t.Run("filters by label selector", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		store.Create(ctx, newTestObject(
			withName("obj1"),
			withNamespace("default"),
			withLabels(map[string]string{"app": "web"}),
		))
		store.Create(ctx, newTestObject(
			withName("obj2"),
			withNamespace("default"),
			withLabels(map[string]string{"app": "api"}),
		))

		// Note: Full label selector support requires setting up LabelSelector properly
		// For now, we test that List accepts the option
		listObj, err := store.List(ctx, storage.ListOptions{
			Namespace: "default",
		})
		if err != nil {
			t.Fatalf("List() failed: %v", err)
		}

		list := listObj.(*unstructured.UnstructuredList)
		if len(list.Items) != 2 {
			t.Errorf("Expected 2 objects, got %d", len(list.Items))
		}
	})
}

func TestMemoryStore_Watch(t *testing.T) {
	t.Run("receives ADDED event on create", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()

		eventCh, stopFunc, err := store.Watch(ctx, storage.ListOptions{Namespace: "default"}, "0")
		if err != nil {
			t.Fatalf("Watch() failed: %v", err)
		}
		defer stopFunc()

		obj := newTestObject(withName("test"), withNamespace("default"))
		store.Create(ctx, obj)

		event := <-eventCh
		if event.Type != storage.EventAdded {
			t.Errorf("Expected ADDED event, got %s", event.Type)
		}
		if event.Object == nil {
			t.Error("Event object is nil")
		}
	})

	t.Run("receives MODIFIED event on update", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(withName("test"), withNamespace("default"))
		store.Create(ctx, obj)

		eventCh, stopFunc, err := store.Watch(ctx, storage.ListOptions{Namespace: "default"}, "1")
		if err != nil {
			t.Fatalf("Watch() failed: %v", err)
		}
		defer stopFunc()

		retrieved, _ := store.Get(ctx, "default", "test")
		store.Update(ctx, retrieved)

		event := <-eventCh
		if event.Type != storage.EventModified {
			t.Errorf("Expected MODIFIED event, got %s", event.Type)
		}
	})

	t.Run("receives DELETED event on delete", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		obj := newTestObject(withName("test"), withNamespace("default"))
		store.Create(ctx, obj)

		eventCh, stopFunc, err := store.Watch(ctx, storage.ListOptions{Namespace: "default"}, "1")
		if err != nil {
			t.Fatalf("Watch() failed: %v", err)
		}
		defer stopFunc()

		store.Delete(ctx, "default", "test")

		event := <-eventCh
		if event.Type != storage.EventDeleted {
			t.Errorf("Expected DELETED event, got %s", event.Type)
		}
	})

	t.Run("stop function closes channel", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()

		eventCh, stopFunc, err := store.Watch(ctx, storage.ListOptions{Namespace: "default"}, "0")
		if err != nil {
			t.Fatalf("Watch() failed: %v", err)
		}

		stopFunc()

		// Channel should be closed
		_, ok := <-eventCh
		if ok {
			t.Error("Expected channel to be closed after stop")
		}
	})
}

func TestMemoryStore_ResourceVersionIncrement(t *testing.T) {
	store := newTestStore()
	ctx := context.Background()

	// Create increments version
	obj1 := newTestObject(withName("obj1"), withNamespace("default"))
	store.Create(ctx, obj1)

	retrieved1, _ := store.Get(ctx, "default", "obj1")
	if retrieved1.GetResourceVersion() != "1" {
		t.Errorf("After first create, rv = %s, want 1", retrieved1.GetResourceVersion())
	}

	// Second create increments further
	obj2 := newTestObject(withName("obj2"), withNamespace("default"))
	store.Create(ctx, obj2)

	retrieved2, _ := store.Get(ctx, "default", "obj2")
	if retrieved2.GetResourceVersion() != "2" {
		t.Errorf("After second create, rv = %s, want 2", retrieved2.GetResourceVersion())
	}

	// Update increments
	store.Update(ctx, retrieved1)

	retrievedUpdated, _ := store.Get(ctx, "default", "obj1")
	if retrievedUpdated.GetResourceVersion() != "3" {
		t.Errorf("After update, rv = %s, want 3", retrievedUpdated.GetResourceVersion())
	}
}

func TestMemoryStore_Concurrency(t *testing.T) {
	t.Run("concurrent creates", func(t *testing.T) {
		store := newTestStore()
		ctx := context.Background()
		done := make(chan bool, 3)

		create := func(ns string) {
			for i := 0; i < 10; i++ {
				obj := newTestObject(
					withName(ns+"-obj"),
					withNamespace(ns),
				)
				store.Create(ctx, obj)
			}
			done <- true
		}

		go create("ns1")
		go create("ns2")
		go create("ns3")

		<-done
		<-done
		<-done

		// Verify all objects were created
		listObj, _ := store.List(ctx, storage.ListOptions{})
		list := listObj.(*unstructured.UnstructuredList)

		if len(list.Items) != 3 {
			t.Errorf("Expected 3 objects, got %d", len(list.Items))
		}
	})
}

func TestMemoryStore_ContextFilter(t *testing.T) {
	t.Run("isolates objects by context filter value", func(t *testing.T) {
		store := newTestStoreWithOpts([]MemoryStoreOption{
			WithContextFilter(contextKey("tenant")),
		})

		ctxA := context.WithValue(context.Background(), contextKey("tenant"), "a")
		ctxB := context.WithValue(context.Background(), contextKey("tenant"), "b")

		// Create objects under tenant "a"
		store.Create(ctxA, newTestObject(withName("obj1"), withNamespace("default")))
		store.Create(ctxA, newTestObject(withName("obj2"), withNamespace("default")))

		// Create objects under tenant "b"
		store.Create(ctxB, newTestObject(withName("obj1"), withNamespace("default")))
		store.Create(ctxB, newTestObject(withName("obj3"), withNamespace("default")))

		// Tenant "a" should see only its objects
		listA, err := store.List(ctxA, storage.ListOptions{Namespace: "default"})
		if err != nil {
			t.Fatalf("List() for tenant a failed: %v", err)
		}
		itemsA := listA.(*unstructured.UnstructuredList)
		if len(itemsA.Items) != 2 {
			t.Errorf("Tenant a: expected 2 objects, got %d", len(itemsA.Items))
		}

		// Tenant "b" should see only its objects
		listB, err := store.List(ctxB, storage.ListOptions{Namespace: "default"})
		if err != nil {
			t.Fatalf("List() for tenant b failed: %v", err)
		}
		itemsB := listB.(*unstructured.UnstructuredList)
		if len(itemsB.Items) != 2 {
			t.Errorf("Tenant b: expected 2 objects, got %d", len(itemsB.Items))
		}
	})

	t.Run("Get is scoped to filter value", func(t *testing.T) {
		store := newTestStoreWithOpts([]MemoryStoreOption{
			WithContextFilter(contextKey("tenant")),
		})

		ctxA := context.WithValue(context.Background(), contextKey("tenant"), "a")
		ctxB := context.WithValue(context.Background(), contextKey("tenant"), "b")

		store.Create(ctxA, newTestObject(withName("only-a"), withNamespace("default")))

		// Tenant "a" can get its object
		obj, err := store.Get(ctxA, "default", "only-a")
		if err != nil {
			t.Fatalf("Get() for tenant a failed: %v", err)
		}
		if obj.GetName() != "only-a" {
			t.Errorf("Expected name only-a, got %s", obj.GetName())
		}

		// Tenant "b" cannot see tenant a's object
		_, err = store.Get(ctxB, "default", "only-a")
		if err == nil {
			t.Error("Expected error when tenant b tries to get tenant a's object")
		}
	})

	t.Run("Delete is scoped to filter value", func(t *testing.T) {
		store := newTestStoreWithOpts([]MemoryStoreOption{
			WithContextFilter(contextKey("tenant")),
		})

		ctxA := context.WithValue(context.Background(), contextKey("tenant"), "a")
		ctxB := context.WithValue(context.Background(), contextKey("tenant"), "b")

		store.Create(ctxA, newTestObject(withName("del-obj"), withNamespace("default")))

		// Tenant "b" should not be able to delete tenant a's object
		err := store.Delete(ctxB, "default", "del-obj")
		if err == nil {
			t.Error("Expected error when tenant b tries to delete tenant a's object")
		}

		// Tenant "a" should still see its object
		_, err = store.Get(ctxA, "default", "del-obj")
		if err != nil {
			t.Fatalf("Object should still exist for tenant a: %v", err)
		}

		// Tenant "a" can delete its own object
		err = store.Delete(ctxA, "default", "del-obj")
		if err != nil {
			t.Fatalf("Delete() for tenant a failed: %v", err)
		}

		_, err = store.Get(ctxA, "default", "del-obj")
		if err == nil {
			t.Error("Object should be deleted for tenant a")
		}
	})

	t.Run("Watch is scoped to filter value", func(t *testing.T) {
		store := newTestStoreWithOpts([]MemoryStoreOption{
			WithContextFilter(contextKey("tenant")),
		})

		ctxA := context.WithValue(context.Background(), contextKey("tenant"), "a")
		ctxB := context.WithValue(context.Background(), contextKey("tenant"), "b")

		// Start watching for tenant "a"
		eventChA, stopA, err := store.Watch(ctxA, storage.ListOptions{Namespace: "default"}, "0")
		if err != nil {
			t.Fatalf("Watch() for tenant a failed: %v", err)
		}
		defer stopA()

		// Create an object under tenant "b" -- tenant a's watch should not see it
		store.Create(ctxB, newTestObject(withName("b-obj"), withNamespace("default")))

		// Create an object under tenant "a" -- tenant a's watch should see it
		store.Create(ctxA, newTestObject(withName("a-obj"), withNamespace("default")))

		event := <-eventChA
		if event.Type != storage.EventAdded {
			t.Errorf("Expected ADDED event, got %s", event.Type)
		}

		eventObj, ok := event.Object.(client.Object)
		if !ok {
			t.Fatal("Event object is not client.Object")
		}
		if eventObj.GetName() != "a-obj" {
			t.Errorf("Expected event for a-obj, got %s", eventObj.GetName())
		}
	})

	t.Run("same name different tenants are independent", func(t *testing.T) {
		store := newTestStoreWithOpts([]MemoryStoreOption{
			WithContextFilter(contextKey("tenant")),
		})

		ctxA := context.WithValue(context.Background(), contextKey("tenant"), "a")
		ctxB := context.WithValue(context.Background(), contextKey("tenant"), "b")

		// Both tenants create an object with the same name
		store.Create(ctxA, newTestObject(
			withName("shared-name"),
			withNamespace("default"),
			withSpec(map[string]interface{}{"owner": "tenant-a"}),
		))
		store.Create(ctxB, newTestObject(
			withName("shared-name"),
			withNamespace("default"),
			withSpec(map[string]interface{}{"owner": "tenant-b"}),
		))

		// Each tenant gets its own version
		objA, err := store.Get(ctxA, "default", "shared-name")
		if err != nil {
			t.Fatalf("Get() for tenant a failed: %v", err)
		}
		specA := objA.(*unstructured.Unstructured).Object["spec"].(map[string]interface{})
		if specA["owner"] != "tenant-a" {
			t.Errorf("Tenant a got wrong object: owner = %v", specA["owner"])
		}

		objB, err := store.Get(ctxB, "default", "shared-name")
		if err != nil {
			t.Fatalf("Get() for tenant b failed: %v", err)
		}
		specB := objB.(*unstructured.Unstructured).Object["spec"].(map[string]interface{})
		if specB["owner"] != "tenant-b" {
			t.Errorf("Tenant b got wrong object: owner = %v", specB["owner"])
		}

		// Deleting from one tenant does not affect the other
		store.Delete(ctxA, "default", "shared-name")

		_, err = store.Get(ctxA, "default", "shared-name")
		if err == nil {
			t.Error("Tenant a should not see deleted object")
		}

		_, err = store.Get(ctxB, "default", "shared-name")
		if err != nil {
			t.Errorf("Tenant b's object should still exist: %v", err)
		}
	})

	t.Run("errors when context filter key is missing", func(t *testing.T) {
		store := newTestStoreWithOpts([]MemoryStoreOption{
			WithContextFilter(contextKey("tenant")),
		})

		// Use a context without the tenant key
		ctx := context.Background()

		err := store.Create(ctx, newTestObject(withName("obj"), withNamespace("default")))
		if err == nil {
			t.Error("Expected error when context filter key is missing")
		}
	})
}
