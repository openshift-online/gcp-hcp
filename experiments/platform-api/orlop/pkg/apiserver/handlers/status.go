package handlers

import (
	"github.com/thetechnick/orlop/pkg/apiserver/constants"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateStatus handles PUT requests to update only the status subresource.
func (h *ResourceHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, constants.URLParamNamespace)
	name := chi.URLParam(r, constants.URLParamName)
	h.logger.V(1).Info("Update status request", "kind", h.gvk.Kind, "namespace", namespace, "name", name)

	// Parse request body
	var updateMap map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updateMap); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	// Get existing object
	existing, err := h.store.Get(r.Context(), namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get object: %v", err))
		}
		return
	}

	// Check resource version
	updateRV, _ := getResourceVersionFromMap(updateMap)
	existingAccessor, err := meta.Accessor(existing)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to access metadata: %v", err))
		return
	}
	existingRV := existingAccessor.GetResourceVersion()

	if updateRV != "" && updateRV != existingRV {
		writeError(w, http.StatusConflict, fmt.Sprintf("resource version mismatch: expected %s, got %s", existingRV, updateRV))
		return
	}

	// Convert existing object to map to preserve spec
	existingJSON, _ := json.Marshal(existing)
	var existingMap map[string]interface{}
	json.Unmarshal(existingJSON, &existingMap)

	// Replace only the status field
	if status, ok := updateMap["status"]; ok {
		existingMap["status"] = status
	}

	// Convert back to typed object
	updatedJSON, _ := json.Marshal(existingMap)
	obj, err := h.scheme.New(h.gvk)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create object: %v", err))
		return
	}
	if err := json.Unmarshal(updatedJSON, obj); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to unmarshal object: %v", err))
		return
	}
	clientObj := obj.(client.Object)

	// Set GVK
	clientObj.GetObjectKind().SetGroupVersionKind(h.gvk)

	// Update in storage
	if err := h.store.Update(r.Context(), clientObj); err != nil {
		h.logger.Error(err, "Update status failed", "kind", h.gvk.Kind, "namespace", namespace, "name", name)
		if errors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
		} else if errors.IsConflict(err) {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update status: %v", err))
		}
		return
	}

	h.logger.Info("Updated status", "kind", h.gvk.Kind, "namespace", namespace, "name", name)

	// Return updated object
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(clientObj)
}

// getResourceVersionFromMap extracts resource version from metadata map.
func getResourceVersionFromMap(objMap map[string]interface{}) (string, error) {
	metadata, ok := objMap["metadata"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("metadata not found or invalid")
	}

	rv, ok := metadata["resourceVersion"].(string)
	if !ok {
		return "", nil
	}

	return rv, nil
}
