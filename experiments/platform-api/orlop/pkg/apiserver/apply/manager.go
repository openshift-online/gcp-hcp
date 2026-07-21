package apply

import (
	"encoding/json"
	"fmt"

	apiextschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// Manager handles server-side apply operations.
// It tracks field ownership and merges apply configurations with existing objects.
type Manager struct {
	scheme        *runtime.Scheme
	openAPISchema *apiextschema.Structural
	gvk           runtimeschema.GroupVersionKind
	typeConverter managedfields.TypeConverter
	fieldManager  *managedfields.FieldManager
}

// NewManager creates a new apply manager.
func NewManager(scheme *runtime.Scheme, openAPISchema *apiextschema.Structural, gvk runtimeschema.GroupVersionKind) (*Manager, error) {
	// Convert structural schema to OpenAPI v3 schema for managedfields
	openAPIV3 := structuralToOpenAPIV3(openAPISchema, gvk)

	// The TypeConverter needs schemas keyed by a model name
	// For CRDs, the convention is to use the GVK path format
	var schemaKey string
	if gvk.Group == "" {
		schemaKey = fmt.Sprintf("%s.%s", gvk.Version, gvk.Kind)
	} else {
		schemaKey = fmt.Sprintf("%s/%s.%s", gvk.Group, gvk.Version, gvk.Kind)
	}

	// Create type converter for managedfields
	typeConverter, err := managedfields.NewTypeConverter(
		map[string]*spec.Schema{
			schemaKey: openAPIV3,
		},
		false, // preserveUnknownFields
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create type converter: %w", err)
	}

	// Create field manager using default CRD field manager
	fieldMgr, err := managedfields.NewDefaultCRDFieldManager(
		typeConverter,
		scheme,          // ObjectConvertor
		scheme,          // ObjectDefaulter
		scheme,          // ObjectCreater
		gvk,             // GroupVersionKind
		gvk.GroupVersion(), // hub version
		"",              // subresource (empty for main resource)
		nil,             // resetFields
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create field manager: %w", err)
	}

	return &Manager{
		scheme:        scheme,
		openAPISchema: openAPISchema,
		gvk:           gvk,
		typeConverter: typeConverter,
		fieldManager:  fieldMgr,
	}, nil
}

// Apply performs server-side apply merge.
//
// It merges the apply configuration with the current object, tracking field ownership
// and detecting conflicts when multiple field managers try to own the same field.
//
// Parameters:
//   - current: The existing object (nil if creating new object)
//   - applyConfig: The desired state to apply (YAML or JSON)
//   - fieldManager: Identifier for the entity applying changes
//   - force: If true, take ownership of conflicting fields
//
// Returns:
//   - The merged object with updated managedFields
//   - Error if conflicts detected (when force=false) or other failures
func (m *Manager) Apply(
	current client.Object,
	applyConfig []byte,
	fieldManager string,
	force bool,
) (client.Object, error) {
	// Parse apply configuration (support both YAML and JSON)
	applyObj, err := m.parseApplyConfig(applyConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse apply configuration: %w", err)
	}

	// If no current object, this is a create operation
	if current == nil {
		return m.applyCreate(applyObj, fieldManager)
	}

	// Perform apply update with field tracking
	return m.applyUpdate(current, applyObj, fieldManager, force)
}

// parseApplyConfig parses YAML or JSON into a typed object
func (m *Manager) parseApplyConfig(data []byte) (client.Object, error) {
	// First, convert YAML to JSON if needed
	var jsonData []byte
	var err error

	// Try to unmarshal as JSON first
	var testJSON map[string]any
	if err := json.Unmarshal(data, &testJSON); err != nil {
		// Not JSON, try YAML
		jsonData, err = yaml.YAMLToJSON(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse as YAML or JSON: %w", err)
		}
	} else {
		jsonData = data
	}

	// Create typed object from scheme
	obj, err := m.scheme.New(m.gvk)
	if err != nil {
		return nil, fmt.Errorf("failed to create object of type %v: %w", m.gvk, err)
	}

	// Unmarshal JSON into typed object
	if err := json.Unmarshal(jsonData, obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal into typed object: %w", err)
	}

	// Set GVK - this is critical for managedfields to work
	gvks, _, err := m.scheme.ObjectKinds(obj)
	if err != nil || len(gvks) == 0 {
		// GVK not registered, set it explicitly
		obj.GetObjectKind().SetGroupVersionKind(m.gvk)
	} else if !gvks[0].Empty() {
		// Use the registered GVK
		obj.GetObjectKind().SetGroupVersionKind(gvks[0])
	}

	// Ensure it's a client.Object
	clientObj, ok := obj.(client.Object)
	if !ok {
		return nil, fmt.Errorf("object does not implement client.Object")
	}

	// Final check: ensure GVK is actually set
	finalGVK := clientObj.GetObjectKind().GroupVersionKind()
	if finalGVK.Empty() {
		// Force set it
		clientObj.GetObjectKind().SetGroupVersionKind(m.gvk)
	}

	return clientObj, nil
}

// applyCreate handles creation of new objects via apply
func (m *Manager) applyCreate(applyObj client.Object, fieldManager string) (client.Object, error) {
	if applyObj == nil {
		return nil, fmt.Errorf("applyObj is nil")
	}

	// For create operations, managedfields.Apply expects a "live" object (not nil)
	// Create an empty object of the same type to serve as the "current" state
	emptyObj, err := m.scheme.New(m.gvk)
	if err != nil {
		return nil, fmt.Errorf("failed to create empty object: %w", err)
	}

	// Set the GVK on the empty object
	emptyObj.GetObjectKind().SetGroupVersionKind(m.gvk)

	// Use the field manager's Apply with empty current (for create)
	result, err := m.fieldManager.Apply(emptyObj, applyObj, fieldManager, false)
	if err != nil {
		return nil, fmt.Errorf("apply create failed: %w", err)
	}

	// Return as client.Object
	if resultObj, ok := result.(client.Object); ok {
		return resultObj, nil
	}

	return nil, fmt.Errorf("unexpected result type: %T", result)
}

// applyUpdate handles updates to existing objects via apply
func (m *Manager) applyUpdate(current client.Object, applyObj client.Object, fieldManager string, force bool) (client.Object, error) {
	// Ensure GVK is set on current object (storage might not preserve it)
	if current.GetObjectKind().GroupVersionKind().Empty() {
		current.GetObjectKind().SetGroupVersionKind(m.gvk)
	}

	// Use the field manager to apply changes
	result, err := m.fieldManager.Apply(current, applyObj, fieldManager, force)
	if err != nil {
		// Check for conflict errors
		if errors.IsConflict(err) {
			return nil, err
		}
		return nil, fmt.Errorf("apply failed: %w", err)
	}

	// Return as client.Object
	if resultObj, ok := result.(client.Object); ok {
		return resultObj, nil
	}

	return nil, fmt.Errorf("unexpected result type: %T", result)
}


// structuralToOpenAPIV3 converts a structural schema to OpenAPI v3 schema
func structuralToOpenAPIV3(structural *apiextschema.Structural, gvk runtimeschema.GroupVersionKind) *spec.Schema {
	if structural == nil {
		return &spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
			},
		}
	}

	schema := &spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type:        []string{structural.Type},
			Description: structural.Description,
		},
	}

	if structural.Properties != nil {
		schema.Properties = make(map[string]spec.Schema)
		for name, prop := range structural.Properties {
			propSchema := structuralToOpenAPIV3Nested(&prop)
			schema.Properties[name] = *propSchema
		}
	}

	if structural.Items != nil {
		schema.Items = &spec.SchemaOrArray{
			Schema: structuralToOpenAPIV3Nested(structural.Items),
		}
	}

	// Add x-kubernetes extensions
	if structural.XPreserveUnknownFields {
		schema.VendorExtensible.AddExtension("x-kubernetes-preserve-unknown-fields", true)
	}
	if structural.XEmbeddedResource {
		schema.VendorExtensible.AddExtension("x-kubernetes-embedded-resource", true)
	}
	if structural.XIntOrString {
		schema.VendorExtensible.AddExtension("x-kubernetes-int-or-string", true)
	}

	// CRITICAL: Add x-kubernetes-group-version-kind extension
	// This is required for the managedfields TypeConverter to index the schema by GVK
	schema.VendorExtensible.AddExtension("x-kubernetes-group-version-kind", []interface{}{
		map[string]interface{}{
			"group":   gvk.Group,
			"version": gvk.Version,
			"kind":    gvk.Kind,
		},
	})

	return schema
}

// structuralToOpenAPIV3Nested converts nested structural schemas (without GVK extension)
func structuralToOpenAPIV3Nested(structural *apiextschema.Structural) *spec.Schema {
	if structural == nil {
		return &spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{"object"},
			},
		}
	}

	schema := &spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type:        []string{structural.Type},
			Description: structural.Description,
		},
	}

	if structural.Properties != nil {
		schema.Properties = make(map[string]spec.Schema)
		for name, prop := range structural.Properties {
			propSchema := structuralToOpenAPIV3Nested(&prop)
			schema.Properties[name] = *propSchema
		}
	}

	if structural.Items != nil {
		schema.Items = &spec.SchemaOrArray{
			Schema: structuralToOpenAPIV3Nested(structural.Items),
		}
	}

	// Add x-kubernetes extensions
	if structural.XPreserveUnknownFields {
		schema.VendorExtensible.AddExtension("x-kubernetes-preserve-unknown-fields", true)
	}
	if structural.XEmbeddedResource {
		schema.VendorExtensible.AddExtension("x-kubernetes-embedded-resource", true)
	}
	if structural.XIntOrString {
		schema.VendorExtensible.AddExtension("x-kubernetes-int-or-string", true)
	}

	return schema
}

// Update tracks field ownership for regular PUT/PATCH updates
func (m *Manager) Update(current, updated client.Object, fieldManager string) (client.Object, error) {
	// For regular updates, use Update operation instead of Apply
	result, err := m.fieldManager.Update(current, updated, fieldManager)
	if err != nil {
		return nil, fmt.Errorf("update failed: %w", err)
	}

	// Return as client.Object
	if resultObj, ok := result.(client.Object); ok {
		return resultObj, nil
	}

	return nil, fmt.Errorf("unexpected result type: %T", result)
}
