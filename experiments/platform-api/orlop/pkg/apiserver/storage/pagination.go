package storage

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// ContinueToken represents the pagination state for continuing a list operation.
type ContinueToken struct {
	// Namespace of the last returned object
	Namespace string `json:"ns,omitempty"`
	// Name of the last returned object
	Name string `json:"n"`
	// ResourceVersion at the time of the list (for consistency)
	ResourceVersion string `json:"rv,omitempty"`
}

// EncodeContinueToken encodes a continue token to a string.
func EncodeContinueToken(token *ContinueToken) (string, error) {
	if token == nil {
		return "", nil
	}

	data, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("failed to marshal continue token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

// DecodeContinueToken decodes a continue token from a string.
func DecodeContinueToken(encoded string) (*ContinueToken, error) {
	if encoded == "" {
		return nil, nil
	}

	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid continue token: %w", err)
	}

	var token ContinueToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal continue token: %w", err)
	}

	return &token, nil
}

// ShouldIncludeObject determines if an object should be included based on the continue token.
// Objects are ordered by namespace/name, so we skip objects until we reach the continuation point.
func ShouldIncludeObject(namespace, name string, continueToken *ContinueToken) bool {
	if continueToken == nil {
		return true
	}

	// Compare namespace first
	if namespace < continueToken.Namespace {
		return false
	}
	if namespace > continueToken.Namespace {
		return true
	}

	// Same namespace, compare name
	return name > continueToken.Name
}
