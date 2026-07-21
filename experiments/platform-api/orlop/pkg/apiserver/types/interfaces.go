package types

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
)

// CustomDefaulter defines an interface for setting defaults on API objects.
// Implement this on your API type to run custom defaulting logic
// after schema-based defaults have been applied.
type CustomDefaulter interface {
	Default(ctx context.Context) error
}

// CustomValidator defines an interface for validating API objects.
// Implement this on your API type to run custom validation logic
// after schema-based validation has passed.
type CustomValidator interface {
	ValidateCreate(ctx context.Context) error
	ValidateUpdate(ctx context.Context, oldObj runtime.Object) error
	ValidateDelete(ctx context.Context) error
}
