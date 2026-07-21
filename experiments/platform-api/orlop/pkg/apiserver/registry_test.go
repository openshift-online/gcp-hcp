package apiserver

import (
	"fmt"
	"testing"

	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"github.com/thetechnick/orlop/pkg/apiserver/storage/memory"
	"github.com/thetechnick/orlop/pkg/apiserver/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewResourceRegistry_DefaultStorage(t *testing.T) {
	scheme := runtime.NewScheme()
	registry := NewResourceRegistry(scheme)

	if registry == nil {
		t.Fatal("Expected registry to be created")
	}

	if registry.storageFactory == nil {
		t.Fatal("Expected default storage factory to be set")
	}

	// Verify default factory creates memory stores
	gvk := schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "Test"}
	store, err := registry.storageFactory("test", scheme, gvk)
	if err != nil {
		t.Fatalf("Default factory failed: %v", err)
	}

	if _, ok := store.(*memory.MemoryStore); !ok {
		t.Errorf("Expected MemoryStore, got %T", store)
	}
}

func TestNewResourceRegistry_CustomStorage(t *testing.T) {
	scheme := runtime.NewScheme()

	customCalled := false
	customFactory := func(resourceType string, s *runtime.Scheme, gvk schema.GroupVersionKind) (storage.ResourceStore, error) {
		customCalled = true
		return memory.NewMemoryStore(resourceType, s, gvk), nil
	}

	registry := NewResourceRegistry(scheme, WithStorageFactory(customFactory))

	if registry == nil {
		t.Fatal("Expected registry to be created")
	}

	// Register a resource to trigger factory
	info := types.ResourceInfo{
		GVK:        schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "Test"},
		Plural:     "tests",
		SchemaYAML: "type: object",
	}

	err := registry.Register(info)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !customCalled {
		t.Error("Custom factory was not called")
	}

	store := registry.GetStore("tests")
	if store == nil {
		t.Error("Expected store to be created")
	}
}

func TestRegister_FactoryError(t *testing.T) {
	scheme := runtime.NewScheme()

	failingFactory := func(resourceType string, s *runtime.Scheme, gvk schema.GroupVersionKind) (storage.ResourceStore, error) {
		return nil, fmt.Errorf("factory error")
	}

	registry := NewResourceRegistry(scheme, WithStorageFactory(failingFactory))

	info := types.ResourceInfo{
		GVK:        schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "Test"},
		Plural:     "tests",
		SchemaYAML: "type: object",
	}

	err := registry.Register(info)
	if err == nil {
		t.Error("Expected error from failing factory")
	}

	if err.Error() != "failed to create storage for tests: factory error" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestRegister_StoresResource(t *testing.T) {
	scheme := runtime.NewScheme()
	registry := NewResourceRegistry(scheme)

	info := types.ResourceInfo{
		GVK:        schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "Test"},
		Plural:     "tests",
		SchemaYAML: "type: object",
	}

	err := registry.Register(info)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify resource is stored
	resources := registry.GetResources()
	if len(resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(resources))
	}

	if resources[0].Plural != "tests" {
		t.Errorf("Expected plural 'tests', got %s", resources[0].Plural)
	}

	// Verify store is created
	store := registry.GetStore("tests")
	if store == nil {
		t.Error("Expected store to be created")
	}
}
