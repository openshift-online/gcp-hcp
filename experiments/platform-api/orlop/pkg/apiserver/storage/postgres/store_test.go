package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test helpers using closure pattern for flexible object creation
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

// setupTestDB creates a test database connection and cleans up after the test
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	// Get connection string from environment or use default
	connString := os.Getenv("POSTGRES_TEST_URL")
	if connString == "" {
		connString = "postgres://localhost/orlop_test?sslmode=disable"
	}

	db, err := sql.Open("postgres", connString)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
		return nil, nil
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("PostgreSQL not reachable: %v", err)
		return nil, nil
	}

	// Cleanup function
	cleanup := func() {
		// Drop test tables
		db.Exec("DROP TABLE IF EXISTS resources_testobjects CASCADE")
		db.Exec("DROP TABLE IF EXISTS event_log CASCADE")
		db.Close()
	}

	return db, cleanup
}

// setupTestStore creates a test PostgresStore with cleanup
func setupTestStore(t *testing.T) (*PostgresStore, func()) {
	db, cleanup := setupTestDB(t)
	if db == nil {
		return nil, func() {}
	}

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

	gvk := schema.GroupVersionKind{
		Group:   "test.example.com",
		Version: "v1",
		Kind:    "TestObject",
	}

	store, err := NewPostgresStore(context.Background(), PostgresStoreConfig{
		DB:           db,
		ResourceType: "testobjects",
		Scheme:       scheme,
		GVK:          gvk,
		TableName:    "resources_testobjects",
	})
	if err != nil {
		cleanup()
		t.Fatalf("Failed to create store: %v", err)
	}

	return store, cleanup
}

func TestPostgresStore_Create(t *testing.T) {
	t.Run("creates new object with resourceVersion", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		obj := newTestObject(withName("test-obj"), withNamespace("default"))

		err := store.Create(context.Background(), obj)
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		retrieved, err := store.Get(context.Background(), "default", "test-obj")
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
		store, cleanup := setupTestStore(t)
		defer cleanup()

		obj := newTestObject(withName("duplicate"), withNamespace("default"))

		store.Create(context.Background(), obj)
		err := store.Create(context.Background(), obj)

		if err == nil {
			t.Error("Expected error for duplicate object, got nil")
		}
	})

	t.Run("creates in different namespaces", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		obj1 := newTestObject(withName("obj"), withNamespace("ns1"))
		obj2 := newTestObject(withName("obj"), withNamespace("ns2"))

		if err := store.Create(context.Background(), obj1); err != nil {
			t.Errorf("Create in ns1 failed: %v", err)
		}
		if err := store.Create(context.Background(), obj2); err != nil {
			t.Errorf("Create in ns2 failed: %v", err)
		}

		// Both should exist
		if _, err := store.Get(context.Background(), "ns1", "obj"); err != nil {
			t.Error("Object in ns1 not found")
		}
		if _, err := store.Get(context.Background(), "ns2", "obj"); err != nil {
			t.Error("Object in ns2 not found")
		}
	})

	t.Run("generateName produces a unique name", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		obj := newTestObject(withGenerateName("gen-"), withNamespace("default"))

		err := store.Create(context.Background(), obj)
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

		retrieved, err := store.Get(context.Background(), "default", name)
		if err != nil {
			t.Fatalf("Get() by generated name failed: %v", err)
		}
		if retrieved.GetName() != name {
			t.Errorf("Retrieved name %q != generated name %q", retrieved.GetName(), name)
		}
	})

	t.Run("generateName creates distinct names", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		seen := make(map[string]bool)

		for range 20 {
			obj := newTestObject(withGenerateName("multi-"), withNamespace("default"))
			if err := store.Create(context.Background(), obj); err != nil {
				t.Fatalf("Create() failed: %v", err)
			}
			name := obj.GetName()
			if seen[name] {
				t.Fatalf("Duplicate generated name: %q", name)
			}
			seen[name] = true
		}

		listObj, _ := store.List(context.Background(), storage.ListOptions{Namespace: "default"})
		list := listObj.(*unstructured.UnstructuredList)
		if len(list.Items) != 20 {
			t.Errorf("Expected 20 objects, got %d", len(list.Items))
		}
	})

	t.Run("sets creation timestamp", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		obj := newTestObject(withName("test"), withNamespace("default"))

		store.Create(context.Background(), obj)

		retrieved, _ := store.Get(context.Background(), "default", "test")
		creationTime := retrieved.GetCreationTimestamp()
		if creationTime.IsZero() {
			t.Error("Creation timestamp not set")
		}
	})
}

func TestPostgresStore_Get(t *testing.T) {
	t.Run("gets existing object", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		obj := newTestObject(withName("test"), withNamespace("default"))
		store.Create(context.Background(), obj)

		retrieved, err := store.Get(context.Background(), "default", "test")
		if err != nil {
			t.Fatalf("Get() failed: %v", err)
		}
		if retrieved.GetName() != "test" {
			t.Errorf("Got wrong object: %s", retrieved.GetName())
		}
	})

	t.Run("returns error for non-existent object", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		_, err := store.Get(context.Background(), "default", "missing")
		if err == nil {
			t.Error("Expected error for missing object, got nil")
		}
	})

	t.Run("returns error for wrong namespace", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		obj := newTestObject(withName("test"), withNamespace("default"))
		store.Create(context.Background(), obj)

		_, err := store.Get(context.Background(), "kube-system", "test")
		if err == nil {
			t.Error("Expected error for wrong namespace, got nil")
		}
	})
}

func TestPostgresStore_List(t *testing.T) {
	t.Run("lists objects in namespace", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		store.Create(context.Background(), newTestObject(withName("obj1"), withNamespace("default")))
		store.Create(context.Background(), newTestObject(withName("obj2"), withNamespace("default")))
		store.Create(context.Background(), newTestObject(withName("obj3"), withNamespace("kube-system")))

		listObj, err := store.List(context.Background(), storage.ListOptions{Namespace: "default"})
		if err != nil {
			t.Fatalf("List() failed: %v", err)
		}

		list := listObj.(*unstructured.UnstructuredList)
		if len(list.Items) != 2 {
			t.Errorf("Expected 2 objects, got %d", len(list.Items))
		}
	})

	t.Run("lists all namespaces", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		store.Create(context.Background(), newTestObject(withName("obj1"), withNamespace("default")))
		store.Create(context.Background(), newTestObject(withName("obj2"), withNamespace("kube-system")))
		store.Create(context.Background(), newTestObject(withName("obj3"), withNamespace("kube-public")))

		listObj, err := store.List(context.Background(), storage.ListOptions{})
		if err != nil {
			t.Fatalf("List() failed: %v", err)
		}

		list := listObj.(*unstructured.UnstructuredList)
		if len(list.Items) != 3 {
			t.Errorf("Expected 3 objects, got %d", len(list.Items))
		}
	})

	t.Run("returns empty list for empty store", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		listObj, err := store.List(context.Background(), storage.ListOptions{Namespace: "default"})
		if err != nil {
			t.Fatalf("List() failed: %v", err)
		}

		list := listObj.(*unstructured.UnstructuredList)
		if len(list.Items) != 0 {
			t.Errorf("Expected empty list, got %d objects", len(list.Items))
		}
	})

	t.Run("sets resourceVersion on list", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		store.Create(context.Background(), newTestObject(withName("obj1"), withNamespace("default")))
		store.Create(context.Background(), newTestObject(withName("obj2"), withNamespace("default")))

		listObj, _ := store.List(context.Background(), storage.ListOptions{Namespace: "default"})
		list := listObj.(*unstructured.UnstructuredList)

		if list.GetResourceVersion() == "" {
			t.Error("List resourceVersion not set")
		}
	})
}

func TestPostgresStore_Update(t *testing.T) {
	t.Run("updates existing object", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		obj := newTestObject(
			withName("test"),
			withNamespace("default"),
			withSpec(map[string]interface{}{"field": "original"}),
		)
		store.Create(context.Background(), obj)

		retrieved, _ := store.Get(context.Background(), "default", "test")
		updated := retrieved.DeepCopyObject().(client.Object)
		u := updated.(*unstructured.Unstructured)
		u.Object["spec"] = map[string]interface{}{"field": "updated"}

		err := store.Update(context.Background(), updated)
		if err != nil {
			t.Fatalf("Update() failed: %v", err)
		}

		final, _ := store.Get(context.Background(), "default", "test")
		finalU := final.(*unstructured.Unstructured)
		spec := finalU.Object["spec"].(map[string]interface{})
		if spec["field"] != "updated" {
			t.Errorf("Update did not persist: got %v", spec["field"])
		}
	})

	t.Run("increments resourceVersion", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		obj := newTestObject(withName("test"), withNamespace("default"))
		store.Create(context.Background(), obj)

		retrieved, _ := store.Get(context.Background(), "default", "test")
		initialRV := retrieved.GetResourceVersion()

		store.Update(context.Background(), retrieved)

		updated, _ := store.Get(context.Background(), "default", "test")
		if updated.GetResourceVersion() == initialRV {
			t.Error("ResourceVersion not incremented after update")
		}
	})

	t.Run("returns error for non-existent object", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		obj := newTestObject(withName("missing"), withNamespace("default"))

		err := store.Update(context.Background(), obj)
		if err == nil {
			t.Error("Expected error for missing object, got nil")
		}
	})
}

func TestPostgresStore_Delete(t *testing.T) {
	t.Run("deletes existing object", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		obj := newTestObject(withName("test"), withNamespace("default"))
		store.Create(context.Background(), obj)

		err := store.Delete(context.Background(), "default", "test")
		if err != nil {
			t.Fatalf("Delete() failed: %v", err)
		}

		_, err = store.Get(context.Background(), "default", "test")
		if err == nil {
			t.Error("Object still exists after delete")
		}
	})

	t.Run("returns error for non-existent object", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		err := store.Delete(context.Background(), "default", "missing")
		if err == nil {
			t.Error("Expected error for missing object, got nil")
		}
	})
}

func TestPostgresStore_ResourceVersionIncrement(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create increments version
	obj1 := newTestObject(withName("obj1"), withNamespace("default"))
	store.Create(context.Background(), obj1)

	retrieved1, _ := store.Get(context.Background(), "default", "obj1")
	if retrieved1.GetResourceVersion() != "1" {
		t.Errorf("After first create, rv = %s, want 1", retrieved1.GetResourceVersion())
	}

	// Second create increments further
	obj2 := newTestObject(withName("obj2"), withNamespace("default"))
	store.Create(context.Background(), obj2)

	retrieved2, _ := store.Get(context.Background(), "default", "obj2")
	if retrieved2.GetResourceVersion() != "2" {
		t.Errorf("After second create, rv = %s, want 2", retrieved2.GetResourceVersion())
	}

	// Update increments
	store.Update(context.Background(), retrieved1)

	retrievedUpdated, _ := store.Get(context.Background(), "default", "obj1")
	if retrievedUpdated.GetResourceVersion() != "3" {
		t.Errorf("After update, rv = %s, want 3", retrievedUpdated.GetResourceVersion())
	}
}

func TestPostgresStore_Persistence(t *testing.T) {
	t.Run("data persists across store instances", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		scheme := runtime.NewScheme()
		gv := schema.GroupVersion{Group: "test.example.com", Version: "v1"}
		scheme.AddKnownTypeWithName(gv.WithKind("TestObject"), &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(gv.WithKind("TestObjectList"), &unstructured.UnstructuredList{})
		gvk := schema.GroupVersionKind{Group: "test.example.com", Version: "v1", Kind: "TestObject"}

		// Create first store and add object
		store1, _ := NewPostgresStore(context.Background(), PostgresStoreConfig{
			DB:           db,
			ResourceType: "testobjects",
			Scheme:       scheme,
			GVK:          gvk,
			TableName:    "resources_testobjects",
		})

		obj := newTestObject(withName("persistent"), withNamespace("default"))
		store1.Create(context.Background(), obj)

		// Create second store (simulates restart)
		store2, _ := NewPostgresStore(context.Background(), PostgresStoreConfig{
			DB:           db,
			ResourceType: "testobjects",
			Scheme:       scheme,
			GVK:          gvk,
			TableName:    "resources_testobjects",
		})

		// Object should still exist
		retrieved, err := store2.Get(context.Background(), "default", "persistent")
		if err != nil {
			t.Fatalf("Object not persisted: %v", err)
		}
		if retrieved.GetName() != "persistent" {
			t.Error("Retrieved wrong object")
		}
	})
}

func TestPostgresStore_Concurrency(t *testing.T) {
	t.Run("concurrent creates in different namespaces", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()

		done := make(chan bool, 3)

		create := func(ns string) {
			for i := 0; i < 5; i++ {
				obj := newTestObject(
					withName(ns+"-obj"),
					withNamespace(ns),
				)
				store.Create(context.Background(), obj)
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
		listObj, _ := store.List(context.Background(), storage.ListOptions{})
		list := listObj.(*unstructured.UnstructuredList)

		if len(list.Items) != 3 {
			t.Errorf("Expected 3 objects, got %d", len(list.Items))
		}
	})
}

func TestPostgresStore_SchemaCreation(t *testing.T) {
	t.Run("creates table and indexes", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		scheme := runtime.NewScheme()
		gv := schema.GroupVersion{Group: "test.example.com", Version: "v1"}
		scheme.AddKnownTypeWithName(gv.WithKind("TestObject"), &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(gv.WithKind("TestObjectList"), &unstructured.UnstructuredList{})
		gvk := schema.GroupVersionKind{Group: "test.example.com", Version: "v1", Kind: "TestObject"}

		_, err := NewPostgresStore(context.Background(), PostgresStoreConfig{
			DB:           db,
			ResourceType: "testobjects",
			Scheme:       scheme,
			GVK:          gvk,
		})
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}

		// Verify table exists
		var exists bool
		err = db.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_name = 'resources_testobjects'
			)
		`).Scan(&exists)
		if err != nil || !exists {
			t.Error("Table was not created")
		}

		// Verify indexes exist
		var indexCount int
		err = db.QueryRow(`
			SELECT COUNT(*) FROM pg_indexes
			WHERE tablename = 'resources_testobjects'
		`).Scan(&indexCount)
		if err != nil || indexCount < 3 {
			t.Errorf("Expected at least 3 indexes, got %d", indexCount)
		}
	})
}
