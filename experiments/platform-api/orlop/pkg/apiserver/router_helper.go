package apiserver

import (
	"fmt"

	"github.com/thetechnick/orlop/pkg/apiserver/conversion"
	"github.com/thetechnick/orlop/pkg/apiserver/handlers"
	"k8s.io/apimachinery/pkg/runtime"
)

// createConvertingHandlerWithSharedStore creates a converting handler that uses the private registry's store
// but the public registry's schema and types.
func createConvertingHandlerWithSharedStore(publicRegistry *ResourceRegistry, privateRegistry *ResourceRegistry, converter *conversion.Converter, privateScheme *runtime.Scheme, publicRes ResourceInfo) (interface{}, error) {
	// Get store from private registry (shared storage)
	store := privateRegistry.GetStore(publicRes.Plural)
	if store == nil {
		return nil, fmt.Errorf("no store found for resource %s in private registry", publicRes.Plural)
	}

	// Create schema processor from public resource schema
	processor, err := publicRegistry.createProcessor(publicRes.SchemaYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to create processor for %s: %w", publicRes.Plural, err)
	}

	// Create converting handler
	handler := handlers.NewConvertingResourceHandler(
		store,         // Store from private registry
		processor,     // Processor from public schema
		converter,     // Converter between public and private
		publicRes.GVK, // Public GVK
		publicRes.Plural,
		publicRegistry.scheme, // Public scheme
		privateScheme,         // Private scheme
		publicRegistry.logger.WithValues("resource", publicRes.Plural),
	)

	return handler, nil
}
