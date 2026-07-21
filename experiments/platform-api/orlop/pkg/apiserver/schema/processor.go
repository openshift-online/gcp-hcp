package schema

import (
	"fmt"

	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	apiextvalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Processor wraps schema operations (pruning, defaulting, validation) for a resource type.
type Processor struct {
	structural *schema.Structural
	validator  apiextvalidation.SchemaValidator
}

// NewProcessor creates a new schema processor from a structural schema and JSONSchemaProps.
func NewProcessor(structural *schema.Structural, props *apiext.JSONSchemaProps) (*Processor, error) {
	if structural == nil {
		return nil, fmt.Errorf("structural schema cannot be nil")
	}

	// Create validator from JSONSchemaProps
	validator, _, err := apiextvalidation.NewSchemaValidator(props)
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}

	return &Processor{
		structural: structural,
		validator:  validator,
	}, nil
}

// Process applies pruning, defaulting, and validation to an object.
// The object should be a map[string]interface{} representing the JSON object.
func (p *Processor) Process(obj interface{}) field.ErrorList {
	// 1. Prune unknown fields
	pruning.Prune(obj, p.structural, true) // true = isResourceRoot

	// 2. Apply defaults
	defaulting.Default(obj, p.structural)

	// 3. Validate
	result := p.validator.Validate(obj)
	if result.IsValid() {
		return nil
	}

	// Convert validation errors to field.ErrorList
	var errs field.ErrorList
	for _, err := range result.Errors {
		errs = append(errs, field.Invalid(
			field.NewPath(""),
			"",
			err.Error(),
		))
	}

	return errs
}

// GetValidationSchema returns the validation schema (JSONSchemaProps).
func (p *Processor) GetValidationSchema() *apiext.JSONSchemaProps {
	// Convert structural schema back to JSONSchemaProps
	// This is needed for apply manager
	return structuralToJSONSchemaProps(p.structural)
}

// structuralToJSONSchemaProps converts a structural schema to JSONSchemaProps
func structuralToJSONSchemaProps(s *schema.Structural) *apiext.JSONSchemaProps {
	if s == nil {
		return &apiext.JSONSchemaProps{}
	}

	props := &apiext.JSONSchemaProps{
		Type:        s.Type,
		Description: s.Description,
	}

	if s.Properties != nil {
		props.Properties = make(map[string]apiext.JSONSchemaProps)
		for name, prop := range s.Properties {
			props.Properties[name] = *structuralToJSONSchemaProps(&prop)
		}
	}

	if s.Items != nil {
		props.Items = &apiext.JSONSchemaPropsOrArray{
			Schema: structuralToJSONSchemaProps(s.Items),
		}
	}

	return props
}
