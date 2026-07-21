package conversion

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

const DefaultPrivatePrefix = "private.orlop.thetechnick.ninja/"

// Converter handles conversion between private and public API types using scheme conversion.
type Converter struct {
	publicScheme  *runtime.Scheme
	privateScheme *runtime.Scheme
	privatePrefix string
}

// NewConverter creates a new converter.
// privatePrefix controls which labels, annotations, and condition types are
// stripped during private-to-public conversion. Pass "" to use DefaultPrivatePrefix.
func NewConverter(publicScheme, privateScheme *runtime.Scheme, privatePrefix string) *Converter {
	if privatePrefix == "" {
		privatePrefix = DefaultPrivatePrefix
	}
	return &Converter{
		publicScheme:  publicScheme,
		privateScheme: privateScheme,
		privatePrefix: privatePrefix,
	}
}

// PrivateToPublic converts a private API object to its public representation.
// Uses JSON round-trip for conversion since both types have the same GVK.
// Filters out private labels, annotations, and conditions.
func (c *Converter) PrivateToPublic(private runtime.Object) (runtime.Object, error) {
	// Get the GVK from the private object
	gvk := private.GetObjectKind().GroupVersionKind()

	// Marshal private object to JSON
	jsonData, err := json.Marshal(private)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private object: %w", err)
	}

	// Create a new public object of the same GVK
	public, err := c.publicScheme.New(gvk)
	if err != nil {
		return nil, fmt.Errorf("failed to create public object for %s: %w", gvk, err)
	}

	// Unmarshal into public object (JSON will only populate fields that exist in public type)
	if err := json.Unmarshal(jsonData, public); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to public object: %w", err)
	}

	// Filter private labels and annotations
	c.filterPrivateMetadata(public)

	// Filter private conditions
	c.filterPrivateConditions(public)

	// Preserve GVK
	public.GetObjectKind().SetGroupVersionKind(gvk)

	return public, nil
}

// filterPrivateMetadata removes labels and annotations with the configured private prefix.
func (c *Converter) filterPrivateMetadata(obj runtime.Object) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return
	}

	// Filter labels
	labels := accessor.GetLabels()
	if labels != nil {
		filtered := make(map[string]string)
		for k, v := range labels {
			if !strings.HasPrefix(k, c.privatePrefix) {
				filtered[k] = v
			}
		}
		accessor.SetLabels(filtered)
	}

	// Filter annotations
	annotations := accessor.GetAnnotations()
	if annotations != nil {
		filtered := make(map[string]string)
		for k, v := range annotations {
			if !strings.HasPrefix(k, c.privatePrefix) {
				filtered[k] = v
			}
		}
		accessor.SetAnnotations(filtered)
	}
}

// filterPrivateConditions removes conditions with types matching the configured private prefix.
func (c *Converter) filterPrivateConditions(obj runtime.Object) {
	// Convert to map to access status.conditions
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return
	}

	var objMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &objMap); err != nil {
		return
	}

	// Check if status.conditions exists
	status, ok := objMap["status"].(map[string]interface{})
	if !ok {
		return
	}

	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		return
	}

	// Filter conditions - handle both string arrays and object arrays
	var filtered []interface{}
	for _, cond := range conditions {
		// Try as string first (simple condition type)
		if condStr, ok := cond.(string); ok {
			// Only include conditions that don't have the private prefix
			if !strings.HasPrefix(condStr, c.privatePrefix) {
				filtered = append(filtered, cond)
			}
			continue
		}

		// Try as object with type field (complex condition)
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		condType, ok := condMap["type"].(string)
		if !ok {
			continue
		}
		// Only include conditions that don't have the private prefix
		if !strings.HasPrefix(condType, c.privatePrefix) {
			filtered = append(filtered, cond)
		}
	}

	// Update conditions in the status
	status["conditions"] = filtered

	// Marshal back and unmarshal into the object
	filteredJSON, err := json.Marshal(objMap)
	if err != nil {
		return
	}

	json.Unmarshal(filteredJSON, obj)
}

// PublicToPrivate converts a public API object to its private representation.
// Uses JSON round-trip for conversion.
// The existing parameter can be used to preserve internal fields.
func (c *Converter) PublicToPrivate(public runtime.Object, existing runtime.Object) (runtime.Object, error) {
	// Get the GVK from the public object
	gvk := public.GetObjectKind().GroupVersionKind()

	// Marshal public object to JSON
	jsonData, err := json.Marshal(public)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public object: %w", err)
	}

	// Create a new private object of the same GVK
	private, err := c.privateScheme.New(gvk)
	if err != nil {
		return nil, fmt.Errorf("failed to create private object for %s: %w", gvk, err)
	}

	// If existing object provided, start with it to preserve internal fields
	if existing != nil {
		// Marshal existing to get all fields
		existingJSON, err := json.Marshal(existing)
		if err == nil {
			// Unmarshal existing into private first
			json.Unmarshal(existingJSON, private)
		}
	}

	// Unmarshal public data into private (will overwrite public fields)
	if err := json.Unmarshal(jsonData, private); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to private object: %w", err)
	}

	// Preserve GVK
	private.GetObjectKind().SetGroupVersionKind(gvk)

	return private, nil
}
