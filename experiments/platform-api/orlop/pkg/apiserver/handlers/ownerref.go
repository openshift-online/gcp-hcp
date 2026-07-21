package handlers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// validateOwnerReferences validates that all owner references point to existing objects
// in the same namespace. Cross-namespace owner references are not allowed.
func (h *ResourceHandler) validateOwnerReferences(ctx context.Context, obj client.Object) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	ownerRefs := accessor.GetOwnerReferences()
	if len(ownerRefs) == 0 {
		return nil
	}

	namespace := accessor.GetNamespace()

	for _, ownerRef := range ownerRefs {
		_, err := h.store.Get(ctx, namespace, ownerRef.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				return errors.NewBadRequest(fmt.Sprintf(
					"ownerReference %s/%s (kind=%s) does not exist",
					namespace, ownerRef.Name, ownerRef.Kind))
			}
			return fmt.Errorf("failed to verify owner %s: %w", ownerRef.Name, err)
		}
	}

	return nil
}

// validateOwnerReferencesFromMap checks that no owner reference in the raw JSON map
// points to a different namespace than the object.
func validateOwnerReferencesFromMap(namespace string, objMap map[string]interface{}) error {
	metadata, ok := objMap["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}
	ownerRefsRaw, ok := metadata["ownerReferences"]
	if !ok {
		return nil
	}
	ownerRefs, ok := ownerRefsRaw.([]interface{})
	if !ok {
		return nil
	}
	for _, refRaw := range ownerRefs {
		ref, ok := refRaw.(map[string]interface{})
		if !ok {
			continue
		}
		ownerNS, ok := ref["namespace"]
		if !ok {
			continue
		}
		nsStr, _ := ownerNS.(string)
		if nsStr != "" && nsStr != namespace {
			name, _ := ref["name"].(string)
			kind, _ := ref["kind"].(string)
			return errors.NewBadRequest(fmt.Sprintf(
				"ownerReference %s/%s (kind=%s) is in a different namespace than the object (%s), cross-namespace owner references are not allowed",
				nsStr, name, kind, namespace))
		}
	}
	return nil
}

// removeOwnerReferencesFromDependents removes the specified owner from all dependent objects.
// This is used for orphan deletion where dependents should survive owner deletion.
func (h *ResourceHandler) removeOwnerReferencesFromDependents(ctx context.Context, namespace, name string, ownerUID string) error {
	// List all objects
	list, err := h.store.List(ctx, storage.ListOptions{Namespace: namespace})
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return fmt.Errorf("failed to extract list: %w", err)
	}

	// Find and update dependents
	for _, item := range items {
		obj, ok := item.(client.Object)
		if !ok {
			continue
		}

		accessor, err := meta.Accessor(obj)
		if err != nil {
			continue
		}

		ownerRefs := accessor.GetOwnerReferences()
		if len(ownerRefs) == 0 {
			continue
		}

		// Check if this object references the owner being deleted
		updated := false
		newOwnerRefs := []metav1.OwnerReference{}
		for _, ref := range ownerRefs {
			if ref.Name == name && string(ref.UID) == ownerUID {
				// Skip this owner reference (orphan the object)
				updated = true
				h.logger.Info("Orphaning dependent object",
					"dependent", accessor.GetName(),
					"owner", name)
			} else {
				newOwnerRefs = append(newOwnerRefs, ref)
			}
		}

		if updated {
			accessor.SetOwnerReferences(newOwnerRefs)
			obj.GetObjectKind().SetGroupVersionKind(h.gvk)
			if err := h.store.Update(ctx, obj); err != nil {
				h.logger.Error(err, "Failed to orphan dependent",
					"dependent", accessor.GetName(),
					"owner", name)
			}
		}
	}

	return nil
}

// deleteDependents deletes all objects that have the specified owner in their ownerReferences.
// This is used for foreground and background cascade deletion.
func (h *ResourceHandler) deleteDependents(ctx context.Context, namespace, name string, ownerUID string) error {
	// List all objects
	list, err := h.store.List(ctx, storage.ListOptions{Namespace: namespace})
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return fmt.Errorf("failed to extract list: %w", err)
	}

	// Find and delete dependents
	for _, item := range items {
		obj, ok := item.(client.Object)
		if !ok {
			continue
		}

		accessor, err := meta.Accessor(obj)
		if err != nil {
			continue
		}

		ownerRefs := accessor.GetOwnerReferences()
		if len(ownerRefs) == 0 {
			continue
		}

		// Check if this object references the owner being deleted
		hasOwner := false
		for _, ref := range ownerRefs {
			if ref.Name == name && string(ref.UID) == ownerUID {
				hasOwner = true
				break
			}
		}

		if hasOwner {
			h.logger.Info("Cascade deleting dependent object",
				"dependent", accessor.GetName(),
				"owner", name)

			if err := h.store.Delete(ctx, namespace, accessor.GetName()); err != nil {
				if !errors.IsNotFound(err) {
					h.logger.Error(err, "Failed to cascade delete dependent",
						"dependent", accessor.GetName(),
						"owner", name)
				}
			}
		}
	}

	return nil
}
