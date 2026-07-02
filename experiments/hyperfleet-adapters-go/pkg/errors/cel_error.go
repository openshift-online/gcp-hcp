package errors

import (
	"errors"
	"fmt"
)

// -----------------------------------------------------------------------------
// CEL Error Types
// -----------------------------------------------------------------------------

// CELErrorType represents the type of CEL error
type CELErrorType string

const (
	CELErrorTypeParse   CELErrorType = "parse"
	CELErrorTypeProgram CELErrorType = "program"
	CELErrorTypeEval    CELErrorType = "evaluation"
)

// CELError represents an error during CEL expression processing
type CELError struct {
	// Expression is the CEL expression that caused the error
	Expression string
	// Reason provides a human-readable error description
	Reason string
	// Err is the underlying error
	Err error
	// Type is the error type (parse, program, evaluation)
	Type CELErrorType
}

// Error implements the error interface
func (e *CELError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("CEL %s error for expression %q: %s: %v", e.Type, e.Expression, e.Reason, e.Err)
	}
	return fmt.Sprintf("CEL %s error for expression %q: %s", e.Type, e.Expression, e.Reason)
}

// Unwrap returns the underlying error for errors.Is/As support
func (e *CELError) Unwrap() error {
	return e.Err
}

// NewCELParseError creates a new CEL parse error
func NewCELParseError(expression string, err error) *CELError {
	return &CELError{
		Type:       CELErrorTypeParse,
		Expression: expression,
		Reason:     "failed to parse expression",
		Err:        err,
	}
}

// NewCELProgramError creates a new CEL program creation error
func NewCELProgramError(expression string, err error) *CELError {
	return &CELError{
		Type:       CELErrorTypeProgram,
		Expression: expression,
		Reason:     "failed to create program",
		Err:        err,
	}
}

// NewCELEvalError creates a new CEL evaluation error
func NewCELEvalError(expression string, err error) *CELError {
	if err == nil {
		return nil
	}
	return &CELError{
		Type:       CELErrorTypeEval,
		Expression: expression,
		Reason:     err.Error(),
		Err:        err,
	}
}

// IsCELError checks if an error is a CELError and returns it
func IsCELError(err error) (*CELError, bool) {
	var celErr *CELError
	if errors.As(err, &celErr) {
		return celErr, true
	}
	return nil, false
}

// IsCELParseError checks if the error is a CEL parse error
func (e *CELError) IsParse() bool {
	return e.Type == CELErrorTypeParse
}

// IsCELProgramError checks if the error is a CEL program error
func (e *CELError) IsProgram() bool {
	return e.Type == CELErrorTypeProgram
}

// IsCELEvalError checks if the error is a CEL evaluation error
func (e *CELError) IsEval() bool {
	return e.Type == CELErrorTypeEval
}

// -----------------------------------------------------------------------------
// CEL Environment Error
// -----------------------------------------------------------------------------

// CELEnvError represents an error when creating a CEL environment
type CELEnvError struct {
	Err    error
	Reason string
}

// Error implements the error interface
func (e *CELEnvError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("failed to create CEL environment: %s: %v", e.Reason, e.Err)
	}
	return fmt.Sprintf("failed to create CEL environment: %s", e.Reason)
}

// Unwrap returns the underlying error
func (e *CELEnvError) Unwrap() error {
	return e.Err
}

// NewCELEnvError creates a new CEL environment error
func NewCELEnvError(reason string, err error) *CELEnvError {
	return &CELEnvError{
		Reason: reason,
		Err:    err,
	}
}

// -----------------------------------------------------------------------------
// CEL Conversion Errors
// -----------------------------------------------------------------------------

// CELConversionError represents an error during condition to CEL conversion
type CELConversionError struct {
	// Err is the underlying error
	Err error
	// Field is the field being converted (for condition errors)
	Field string
	// Operator is the operator being used
	Operator string
	// ValueType is the type of value that couldn't be converted
	ValueType string
	// Reason provides context about what failed
	Reason string
	// Index is the condition index (for multiple conditions)
	Index int
}

// Error implements the error interface
func (e *CELConversionError) Error() string {
	if e.Index >= 0 {
		return fmt.Sprintf("failed to convert condition %d to CEL: %s", e.Index, e.Reason)
	}
	if e.Operator != "" {
		return fmt.Sprintf("unsupported operator for CEL conversion: %s", e.Operator)
	}
	if e.ValueType != "" {
		return fmt.Sprintf("unsupported type for CEL formatting: %s", e.ValueType)
	}
	return fmt.Sprintf("CEL conversion error: %s", e.Reason)
}

// Unwrap returns the underlying error
func (e *CELConversionError) Unwrap() error {
	return e.Err
}

// NewCELUnsupportedOperatorError creates an error for unsupported operators
func NewCELUnsupportedOperatorError(operator string) *CELConversionError {
	return &CELConversionError{
		Operator: operator,
		Reason:   fmt.Sprintf("operator %q is not supported for CEL conversion", operator),
		Index:    -1,
	}
}

// NewCELUnsupportedTypeError creates an error for unsupported types
func NewCELUnsupportedTypeError(valueType string) *CELConversionError {
	return &CELConversionError{
		ValueType: valueType,
		Reason:    fmt.Sprintf("type %s cannot be formatted as CEL literal", valueType),
		Index:     -1,
	}
}

// NewCELConditionConversionError creates an error for condition conversion failures
func NewCELConditionConversionError(index int, err error) *CELConversionError {
	return &CELConversionError{
		Index:  index,
		Reason: "condition conversion failed",
		Err:    err,
	}
}

// -----------------------------------------------------------------------------
// CEL Type Mismatch Error
// -----------------------------------------------------------------------------

// CELTypeMismatchError represents a type mismatch when evaluating a CEL expression
type CELTypeMismatchError struct {
	Expression   string
	ExpectedType string
	ActualType   string
}

// Error implements the error interface
func (e *CELTypeMismatchError) Error() string {
	return fmt.Sprintf("CEL expression %q returned %s, expected %s",
		e.Expression, e.ActualType, e.ExpectedType)
}

// NewCELTypeMismatchError creates a new CELTypeMismatchError
func NewCELTypeMismatchError(expression, expectedType, actualType string) *CELTypeMismatchError {
	return &CELTypeMismatchError{
		Expression:   expression,
		ExpectedType: expectedType,
		ActualType:   actualType,
	}
}
