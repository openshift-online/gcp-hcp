package handlers

import (
	"context"
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// parentFilterKey is the context key for parent ID filtering.
type parentFilterKey struct{}

// ParentFilter holds the field path and expected value for filtering child resources
// by their parent ID.
type ParentFilter struct {
	// IDField is the dot-separated JSON field path (e.g., "spec.clusterID").
	IDField string
	// ID is the expected parent ID value.
	ID string
}

// WithParentFilter returns a new context with the parent filter set.
func WithParentFilter(ctx context.Context, f ParentFilter) context.Context {
	return context.WithValue(ctx, parentFilterKey{}, f)
}

// parentFilterFromContext returns the ParentFilter from the context, if any.
func parentFilterFromContext(ctx context.Context) (ParentFilter, bool) {
	f, ok := ctx.Value(parentFilterKey{}).(ParentFilter)
	return f, ok
}

// fieldValue navigates a dot-separated path in a JSON-marshalled object and returns
// the string value at that path, or "" if the path does not exist.
func fieldValue(obj runtime.Object, path string) string {
	data, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	parts := strings.Split(path, ".")
	var current interface{} = m
	for _, part := range parts {
		mm, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = mm[part]
	}
	if current == nil {
		return ""
	}
	s, _ := current.(string)
	return s
}

// applyParentFilter removes items from list that do not match the parent filter.
// Returns the original list unchanged if no filter is present in ctx.
func applyParentFilter(ctx context.Context, list runtime.Object) error {
	filter, ok := parentFilterFromContext(ctx)
	if !ok {
		return nil
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return err
	}

	var filtered []runtime.Object
	for _, item := range items {
		if fieldValue(item, filter.IDField) == filter.ID {
			filtered = append(filtered, item)
		}
	}

	return meta.SetList(list, filtered)
}
