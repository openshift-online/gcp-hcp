package apiserver

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/thetechnick/orlop/pkg/apiserver/apply"
	"github.com/thetechnick/orlop/pkg/apiserver/conversion"
	"github.com/thetechnick/orlop/pkg/apiserver/handlers"
	pkgschema "github.com/thetechnick/orlop/pkg/apiserver/schema"
	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"github.com/thetechnick/orlop/pkg/apiserver/storage/memory"
	"github.com/thetechnick/orlop/pkg/apiserver/types"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// ResourceInfo is re-exported from types package for convenience.
type ResourceInfo = types.ResourceInfo

// StorageFactory is a function that creates a storage.ResourceStore for a given resource.
// This allows custom storage backends (PostgreSQL, etc.) to be used instead of the default memory store.
type StorageFactory func(resourceType string, scheme *runtime.Scheme, gvk runtimeschema.GroupVersionKind) (storage.ResourceStore, error)

// ResourceRegistry manages API resource registrations and their stores.
type ResourceRegistry struct {
	resources      []ResourceInfo
	stores         map[string]storage.ResourceStore
	scheme         *runtime.Scheme
	storageFactory StorageFactory
	logger         logr.Logger
}

// RegistryOption configures a ResourceRegistry.
type RegistryOption func(*ResourceRegistry)

// WithStorageFactory configures a custom storage factory.
// If not provided, the default in-memory storage is used.
func WithStorageFactory(factory StorageFactory) RegistryOption {
	return func(r *ResourceRegistry) {
		r.storageFactory = factory
	}
}

// WithLogger configures a logger for the registry.
// If not provided, a discard logger is used.
func WithLogger(logger logr.Logger) RegistryOption {
	return func(r *ResourceRegistry) {
		r.logger = logger
	}
}

// NewResourceRegistry creates a new resource registry.
func NewResourceRegistry(scheme *runtime.Scheme, opts ...RegistryOption) *ResourceRegistry {
	r := &ResourceRegistry{
		resources: []ResourceInfo{},
		stores:    make(map[string]storage.ResourceStore),
		scheme:    scheme,
		// Default to in-memory storage
		storageFactory: func(resourceType string, scheme *runtime.Scheme, gvk runtimeschema.GroupVersionKind) (storage.ResourceStore, error) {
			return memory.NewMemoryStore(resourceType, scheme, gvk), nil
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Register adds a resource to the registry and creates its store using the configured storage factory.
func (r *ResourceRegistry) Register(info ResourceInfo) error {
	r.resources = append(r.resources, info)

	// Create store using the storage factory
	store, err := r.storageFactory(info.Plural, r.scheme, info.GVK)
	if err != nil {
		return fmt.Errorf("failed to create storage for %s: %w", info.Plural, err)
	}

	r.stores[info.Plural] = store
	return nil
}

// GetStore returns the store for a given resource plural name.
func (r *ResourceRegistry) GetStore(plural string) storage.ResourceStore {
	return r.stores[plural]
}

// GetStores returns all stores indexed by resource plural name.
func (r *ResourceRegistry) GetStores() map[string]storage.ResourceStore {
	return r.stores
}

// Resources returns all registered resources.
func (r *ResourceRegistry) Resources() []types.ResourceInfo {
	return r.resources
}

// GetResources returns the internal resource list.
func (r *ResourceRegistry) GetResources() []ResourceInfo {
	return r.resources
}

// CreateHandler creates a ResourceHandler for the given resource info.
func (r *ResourceRegistry) CreateHandler(info ResourceInfo) (*handlers.ResourceHandler, error) {
	// Get store for this resource
	store := r.GetStore(info.Plural)
	if store == nil {
		return nil, fmt.Errorf("no store found for resource %s", info.Plural)
	}

	// Create schema processor
	processor, err := r.createProcessor(info.SchemaYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to create processor for %s: %w", info.Plural, err)
	}

	// Create handler
	handler := handlers.NewResourceHandler(
		store,
		processor,
		info.GVK,
		info.Plural,
		r.scheme,
		r.logger.WithValues("resource", info.Plural),
	)

	// Create and set apply manager for server-side apply support
	structural, err := schema.NewStructural(processor.GetValidationSchema())
	if err != nil {
		r.logger.Info("Failed to create structural schema, server-side apply disabled", "resource", info.Plural, "error", err)
	} else {
		applyMgr, err := apply.NewManager(r.scheme, structural, info.GVK)
		if err != nil {
			r.logger.Info("Failed to create apply manager, server-side apply disabled", "resource", info.Plural, "error", err)
		} else {
			handler.SetApplyManager(applyMgr)
			r.logger.Info("Server-side apply enabled", "resource", info.Plural)
		}
	}

	return handler, nil
}

// CreateConvertingHandler creates a ConvertingResourceHandler for the given resource info.
func (r *ResourceRegistry) CreateConvertingHandler(converter interface{}, privateScheme *runtime.Scheme, info ResourceInfo) (interface{}, error) {
	// Get store for this resource
	store := r.GetStore(info.Plural)
	if store == nil {
		return nil, fmt.Errorf("no store found for resource %s", info.Plural)
	}

	// Create schema processor
	processor, err := r.createProcessor(info.SchemaYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to create processor for %s: %w", info.Plural, err)
	}

	// Create converting handler
	handler := handlers.NewConvertingResourceHandler(
		store,
		processor,
		converter.(*conversion.Converter),
		info.GVK,
		info.Plural,
		r.scheme,      // Public scheme from registry
		privateScheme, // Private scheme passed in
		r.logger.WithValues("resource", info.Plural),
	)

	return handler, nil
}

// createProcessor creates a schema processor from YAML schema.
func (r *ResourceRegistry) createProcessor(schemaYAML string) (*pkgschema.Processor, error) {
	// Parse YAML to v1 JSONSchemaProps
	var propsV1 apiextv1.JSONSchemaProps
	if err := yaml.Unmarshal([]byte(schemaYAML), &propsV1); err != nil {
		return nil, err
	}

	// Convert to internal JSONSchemaProps
	var props apiext.JSONSchemaProps
	if err := apiextv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(&propsV1, &props, nil); err != nil {
		return nil, err
	}

	// Create structural schema
	structural, err := schema.NewStructural(&props)
	if err != nil {
		return nil, err
	}

	// Create processor
	return pkgschema.NewProcessor(structural, &props)
}
