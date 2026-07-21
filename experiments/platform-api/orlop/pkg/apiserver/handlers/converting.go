package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/thetechnick/orlop/pkg/apiserver/constants"
	"github.com/thetechnick/orlop/pkg/apiserver/conversion"
	"github.com/thetechnick/orlop/pkg/apiserver/schema"
	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"github.com/thetechnick/orlop/pkg/apiserver/types"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConvertingResourceHandler wraps a ResourceHandler and performs conversions
// between public and private API types.
type ConvertingResourceHandler struct {
	store         storage.ResourceStore
	processor     *schema.Processor
	converter     *conversion.Converter
	gvk           runtimeschema.GroupVersionKind
	resourceType  string
	publicScheme  *runtime.Scheme // Scheme for public API types
	privateScheme *runtime.Scheme // Scheme for private API types
	logger        logr.Logger
}

// NewConvertingResourceHandler creates a new converting resource handler.
func NewConvertingResourceHandler(
	store storage.ResourceStore,
	processor *schema.Processor,
	converter *conversion.Converter,
	gvk runtimeschema.GroupVersionKind,
	resourceType string,
	publicScheme *runtime.Scheme,
	privateScheme *runtime.Scheme,
	logger logr.Logger,
) *ConvertingResourceHandler {
	if logger.GetSink() == nil {
		logger = logr.Discard()
	}
	return &ConvertingResourceHandler{
		store:         store,
		processor:     processor,
		converter:     converter,
		gvk:           gvk,
		resourceType:  resourceType,
		publicScheme:  publicScheme,
		privateScheme: privateScheme,
		logger:        logger,
	}
}

// Create handles POST requests to create a new resource.
func (h *ConvertingResourceHandler) Create(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, constants.URLParamNamespace)
	h.logger.V(1).Info("Create request (converting)", "kind", h.gvk.Kind, "namespace", namespace)

	// Parse request body as map for schema processing
	var objMap map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&objMap); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if err := validateOwnerReferencesFromMap(namespace, objMap); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid ownerReferences: %v", err))
		return
	}

	// Process object (prune, default, validate) using public schema
	if errs := h.processor.Process(objMap); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("validation failed: %v", errs.ToAggregate()))
		return
	}

	// Convert to public typed object
	objJSON, _ := json.Marshal(objMap)
	publicObj, err := h.publicScheme.New(h.gvk)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create public object: %v", err))
		return
	}
	if err := json.Unmarshal(objJSON, publicObj); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to unmarshal object: %v", err))
		return
	}

	// Set metadata on public object
	accessor, err := meta.Accessor(publicObj)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to access metadata: %v", err))
		return
	}

	name := accessor.GetName()
	if name == "" && accessor.GetGenerateName() == "" {
		writeError(w, http.StatusBadRequest, "metadata.name or metadata.generateName is required")
		return
	}

	accessor.SetNamespace(namespace)
	accessor.SetUID(k8stypes.UID(uuid.New().String()))
	accessor.SetCreationTimestamp(metav1.Time{Time: time.Now()})
	accessor.SetGeneration(1)

	// Set GVK on public object
	publicObj.GetObjectKind().SetGroupVersionKind(h.gvk)

	// Convert public to private (no existing object)
	privateObjRaw, err := h.privateScheme.New(h.gvk)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create private object: %v", err))
		return
	}
	privateObj, err := h.converter.PublicToPrivate(publicObj, privateObjRaw.(client.Object))
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to convert to private: %v", err))
		return
	}

	if d, ok := privateObj.(types.CustomDefaulter); ok {
		if err := d.Default(r.Context()); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("defaulting failed: %v", err))
			return
		}
	}

	if v, ok := privateObj.(types.CustomValidator); ok {
		if err := v.ValidateCreate(r.Context()); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("validation failed: %v", err))
			return
		}
	}

	// Set GVK on private object for storage
	privateObj.GetObjectKind().SetGroupVersionKind(h.gvk)

	// Store private object (cast to client.Object)
	if err := h.store.Create(r.Context(), privateObj.(client.Object)); err != nil {
		h.logger.Error(err, "Create failed (converting)", "kind", h.gvk.Kind, "namespace", namespace, "name", accessor.GetName())
		if errors.IsAlreadyExists(err) {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create object: %v", err))
		}
		return
	}

	name = accessor.GetName()
	h.logger.Info("Created (converting)", "kind", h.gvk.Kind, "namespace", namespace, "name", name)

	// Convert stored private object back to public to get ResourceVersion
	responsePublic, err := h.converter.PrivateToPublic(privateObj)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to convert to public: %v", err))
		return
	}

	// Return public representation
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(responsePublic)
}

// Get handles GET requests to retrieve a single resource.
func (h *ConvertingResourceHandler) Get(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, constants.URLParamNamespace)
	name := chi.URLParam(r, constants.URLParamName)
	h.logger.V(1).Info("Get request (converting)", "kind", h.gvk.Kind, "namespace", namespace, "name", name)

	// Get private object from storage
	privateObj, err := h.store.Get(r.Context(), namespace, name)
	if err != nil {
		h.logger.Error(err, "Get failed (converting)", "kind", h.gvk.Kind, "namespace", namespace, "name", name)
		if errors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get object: %v", err))
		}
		return
	}

	// Convert to public
	publicObj, err := h.converter.PrivateToPublic(privateObj)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to convert to public: %v", err))
		return
	}

	h.logger.V(1).Info("Found (converting)", "kind", h.gvk.Kind, "namespace", namespace, "name", name)

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(publicObj)
}

// List handles GET requests to list resources.
func (h *ConvertingResourceHandler) List(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, constants.URLParamNamespace)
	if namespace == "" {
		h.logger.V(1).Info("List request (converting)", "kind", h.gvk.Kind, "scope", "cluster")
	} else {
		h.logger.V(1).Info("List request (converting)", "kind", h.gvk.Kind, "namespace", namespace)
	}

	// Build list options from query parameters
	opts := storage.ListOptions{
		Namespace: namespace,
	}

	// Parse label selector from query parameter
	if labelSelectorStr := r.URL.Query().Get(constants.QueryParamLabelSelector); labelSelectorStr != "" {
		opts.LabelSelector = labelSelectorStr
	}

	// Check if this is a watch request
	if r.URL.Query().Get(constants.QueryParamWatch) == "true" {
		if namespace == "" {
			h.logger.V(1).Info("Watch request (converting)", "kind", h.gvk.Kind, "scope", "cluster", "uri", r.RequestURI)
		} else {
			h.logger.V(1).Info("Watch request (converting)", "kind", h.gvk.Kind, "namespace", namespace, "uri", r.RequestURI)
		}
		h.handleWatch(w, r, opts)
		return
	}

	// Get private objects list from storage
	privateList, err := h.store.List(r.Context(), opts)
	if err != nil {
		if namespace == "" {
			h.logger.Error(err, "List failed (converting)", "kind", h.gvk.Kind, "scope", "cluster")
		} else {
			h.logger.Error(err, "List failed (converting)", "kind", h.gvk.Kind, "namespace", namespace)
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list objects: %v", err))
		return
	}

	// Extract items from the list using meta.ExtractList
	privateItems, err := meta.ExtractList(privateList)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to extract list items: %v", err))
		return
	}

	// Convert each private object to public
	publicObjects := make([]runtime.Object, 0, len(privateItems))
	for _, privateObj := range privateItems {
		publicObj, err := h.converter.PrivateToPublic(privateObj)
		if err != nil {
			continue // Skip objects that fail conversion
		}
		publicObjects = append(publicObjects, publicObj)
	}

	// Build list response manually as JSON to avoid type conversion issues
	// meta.SetList() requires exact type matches which don't work across packages
	listResponse := map[string]interface{}{
		"apiVersion": h.gvk.GroupVersion().String(),
		"kind":       h.gvk.Kind + "List",
		constants.FieldMetadata: map[string]interface{}{
			"resourceVersion": privateList.GetResourceVersion(),
		},
		"items": publicObjects,
	}

	if namespace == "" {
		h.logger.V(1).Info("Listed (converting)", "kind", h.gvk.Kind, "scope", "cluster", "count", len(publicObjects))
	} else {
		h.logger.V(1).Info("Listed (converting)", "kind", h.gvk.Kind, "namespace", namespace, "count", len(publicObjects))
	}

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(listResponse)
}

// handleWatch handles watch requests using streaming JSON.
func (h *ConvertingResourceHandler) handleWatch(w http.ResponseWriter, r *http.Request, opts storage.ListOptions) {
	config := parseWatchConfig(r)

	h.logger.V(1).Info("Watch parameters (converting)",
		config.allowWatchBookmarks, config.sendInitialEvents, config.resourceVersionMatch, config.timeoutSeconds)

	// Apply timeout if specified
	ctx := applyWatchTimeout(r.Context(), config.timeoutSeconds)

	// Start watch
	eventCh, stop, err := h.store.Watch(ctx, opts, config.resourceVersion)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to start watch: %v", err))
		return
	}
	defer stop()

	// Get current resource version and check if list is empty
	var currentRV string
	var isEmpty bool
	list, err := h.store.List(ctx, opts)
	if err == nil {
		currentRV = list.GetResourceVersion()
		items, _ := meta.ExtractList(list)
		isEmpty = len(items) == 0
	} else {
		currentRV = "0"
		isEmpty = true
	}

	// Set up streaming response
	streamer, err := newWatchStreamer(w, h.gvk, currentRV, isEmpty)
	if err != nil {
		return
	}

	// Create transformer to convert private objects to public
	transformer := func(obj client.Object) (interface{}, error) {
		return h.converter.PrivateToPublic(obj)
	}

	// Stream watch events with transformation
	streamWatch(ctx, streamer, eventCh, config, opts, h.store, transformer)
}

// Update handles PUT requests to update a resource.
func (h *ConvertingResourceHandler) Update(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, constants.URLParamNamespace)
	name := chi.URLParam(r, constants.URLParamName)
	h.logger.V(1).Info("Update request (converting)", "kind", h.gvk.Kind, "namespace", namespace, "name", name)

	// Get existing private object
	existingPrivate, err := h.store.Get(r.Context(), namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get existing object: %v", err))
		}
		return
	}

	// Parse request body
	var objMap map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&objMap); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if err := validateOwnerReferencesFromMap(namespace, objMap); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid ownerReferences: %v", err))
		return
	}

	// Process object (prune, default, validate) using public schema
	if errs := h.processor.Process(objMap); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("validation failed: %v", errs.ToAggregate()))
		return
	}

	// Convert to public typed object
	objJSON, _ := json.Marshal(objMap)
	publicObj, err := h.publicScheme.New(h.gvk)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create public object: %v", err))
		return
	}
	if err := json.Unmarshal(objJSON, publicObj); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to unmarshal object: %v", err))
		return
	}

	// Set metadata
	accessor, err := meta.Accessor(publicObj)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to access metadata: %v", err))
		return
	}

	accessor.SetNamespace(namespace)
	accessor.SetName(name)

	// Convert public to private, preserving internal fields
	privateObj, err := h.converter.PublicToPrivate(publicObj, existingPrivate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to convert to private: %v", err))
		return
	}

	if d, ok := privateObj.(types.CustomDefaulter); ok {
		if err := d.Default(r.Context()); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("defaulting failed: %v", err))
			return
		}
	}

	if v, ok := privateObj.(types.CustomValidator); ok {
		if err := v.ValidateUpdate(r.Context(), existingPrivate); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("validation failed: %v", err))
			return
		}
	}

	// Check if spec changed and increment generation if so
	existingAccessor, _ := meta.Accessor(existingPrivate)
	privateAccessor, _ := meta.Accessor(privateObj)
	if specChanged(existingPrivate, privateObj) {
		privateAccessor.SetGeneration(existingAccessor.GetGeneration() + 1)
	} else {
		privateAccessor.SetGeneration(existingAccessor.GetGeneration())
	}

	// Set GVK
	privateObj.GetObjectKind().SetGroupVersionKind(h.gvk)

	// Update object in storage (cast to client.Object)
	if err := h.store.Update(r.Context(), privateObj.(client.Object)); err != nil {
		h.logger.Error(err, "Update failed (converting)", "kind", h.gvk.Kind, "namespace", namespace, "name", name)
		if errors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
		} else if errors.IsConflict(err) {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update object: %v", err))
		}
		return
	}

	h.logger.Info("Updated (converting)", "kind", h.gvk.Kind, "namespace", namespace, "name", name)

	// Convert back to public for response
	responsePublic, _ := h.converter.PrivateToPublic(privateObj)

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responsePublic)
}

// Patch handles PATCH requests to partially update a resource.
func (h *ConvertingResourceHandler) Patch(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, constants.URLParamNamespace)
	name := chi.URLParam(r, constants.URLParamName)
	contentType := r.Header.Get(constants.HeaderContentType)
	h.logger.V(1).Info("[PATCH-CONVERTING] %s namespace=%s name=%s content-type=%s", h.gvk.Kind, namespace, name, contentType)

	// Get existing private object
	existingPrivate, err := h.store.Get(r.Context(), namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get object: %v", err))
		}
		return
	}

	// Convert to public for patching
	existingPublic, err := h.converter.PrivateToPublic(existingPrivate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to convert to public: %v", err))
		return
	}

	// Read patch body
	patchBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to read patch: %v", err))
		return
	}

	// Convert existing public object to JSON
	existingJSON, err := json.Marshal(existingPublic)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to marshal existing object: %v", err))
		return
	}

	// Apply merge patch (same logic as ResourceHandler)
	patchedJSON, err := jsonMergePatch(existingJSON, patchBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("patch failed: %v", err))
		return
	}

	// Convert to map for schema processing
	var objMap map[string]interface{}
	if err := json.Unmarshal(patchedJSON, &objMap); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to unmarshal patched object: %v", err))
		return
	}

	// Process object (prune, default, validate) using public schema
	if errs := h.processor.Process(objMap); len(errs) > 0 {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("validation failed: %v", errs.ToAggregate()))
		return
	}

	// Convert to public typed object
	objJSON, _ := json.Marshal(objMap)
	publicObj, err := h.publicScheme.New(h.gvk)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create public object: %v", err))
		return
	}
	if err := json.Unmarshal(objJSON, publicObj); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to unmarshal object: %v", err))
		return
	}

	// Set metadata
	accessor, err := meta.Accessor(publicObj)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to access metadata: %v", err))
		return
	}

	accessor.SetNamespace(namespace)
	accessor.SetName(name)

	// Convert public to private
	privateObj, err := h.converter.PublicToPrivate(publicObj, existingPrivate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to convert to private: %v", err))
		return
	}

	if d, ok := privateObj.(types.CustomDefaulter); ok {
		if err := d.Default(r.Context()); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("defaulting failed: %v", err))
			return
		}
	}

	if v, ok := privateObj.(types.CustomValidator); ok {
		if err := v.ValidateUpdate(r.Context(), existingPrivate); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("validation failed: %v", err))
			return
		}
	}

	// Check if spec changed and increment generation if so
	existingAccessor, _ := meta.Accessor(existingPrivate)
	privateAccessor, _ := meta.Accessor(privateObj)
	if specChanged(existingPrivate, privateObj) {
		privateAccessor.SetGeneration(existingAccessor.GetGeneration() + 1)
	} else {
		privateAccessor.SetGeneration(existingAccessor.GetGeneration())
	}

	// Set GVK
	privateObj.GetObjectKind().SetGroupVersionKind(h.gvk)

	// Update object in storage (cast to client.Object)
	if err := h.store.Update(r.Context(), privateObj.(client.Object)); err != nil {
		h.logger.V(1).Info("[PATCH-CONVERTING] %s namespace=%s name=%s error=%v", h.gvk.Kind, namespace, name, err)
		if errors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
		} else if errors.IsConflict(err) {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update object: %v", err))
		}
		return
	}

	h.logger.V(1).Info("[PATCH-CONVERTING] %s namespace=%s name=%s status=patched", h.gvk.Kind, namespace, name)

	// Convert back to public for response
	responsePublic, _ := h.converter.PrivateToPublic(privateObj)

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responsePublic)
}

// Delete handles DELETE requests to delete a resource.
func (h *ConvertingResourceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, constants.URLParamNamespace)
	name := chi.URLParam(r, constants.URLParamName)
	h.logger.V(1).Info("[DELETE-CONVERTING] %s namespace=%s name=%s", h.gvk.Kind, namespace, name)

	existing, err := h.store.Get(r.Context(), namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get object: %v", err))
		}
		return
	}

	if v, ok := existing.(types.CustomValidator); ok {
		if err := v.ValidateDelete(r.Context()); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("validation failed: %v", err))
			return
		}
	}

	if err := h.store.Delete(r.Context(), namespace, name); err != nil {
		h.logger.V(1).Info("[DELETE-CONVERTING] %s namespace=%s name=%s error=%v", h.gvk.Kind, namespace, name, err)
		if errors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete object: %v", err))
		}
		return
	}

	h.logger.V(1).Info("[DELETE-CONVERTING] %s namespace=%s name=%s status=deleted", h.gvk.Kind, namespace, name)

	// Return success status
	status := metav1.Status{
		TypeMeta: metav1.TypeMeta{
			APIVersion: constants.APIVersionV1,
			Kind:       constants.KindStatus,
		},
		Status: metav1.StatusSuccess,
		Code:   http.StatusOK,
	}

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// UpdateStatus handles PUT requests to update only the status subresource.
func (h *ConvertingResourceHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, constants.URLParamNamespace)
	name := chi.URLParam(r, constants.URLParamName)
	h.logger.V(1).Info("[UPDATE-STATUS-CONVERTING] %s namespace=%s name=%s", h.gvk.Kind, namespace, name)

	// Parse request body
	var updateMap map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updateMap); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	// Get existing private object
	existingPrivate, err := h.store.Get(r.Context(), namespace, name)
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
	existingAccessor, err := meta.Accessor(existingPrivate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to access metadata: %v", err))
		return
	}
	existingRV := existingAccessor.GetResourceVersion()

	if updateRV != "" && updateRV != existingRV {
		writeError(w, http.StatusConflict, fmt.Sprintf("resource version mismatch: expected %s, got %s", existingRV, updateRV))
		return
	}

	// Convert existing private to map, update only status
	existingJSON, _ := json.Marshal(existingPrivate)
	var existingMap map[string]interface{}
	json.Unmarshal(existingJSON, &existingMap)

	// Replace only the status field
	if status, ok := updateMap["status"]; ok {
		existingMap["status"] = status
	}

	// Convert back to private object
	updatedJSON, _ := json.Marshal(existingMap)
	updatedPrivateRaw, err := h.privateScheme.New(h.gvk)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create private object: %v", err))
		return
	}
	updatedPrivate := updatedPrivateRaw.(client.Object)
	if err := json.Unmarshal(updatedJSON, updatedPrivate); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to unmarshal object: %v", err))
		return
	}

	// Set GVK
	updatedPrivate.GetObjectKind().SetGroupVersionKind(h.gvk)

	// Update in storage
	if err := h.store.Update(r.Context(), updatedPrivate); err != nil {
		h.logger.V(1).Info("[UPDATE-STATUS-CONVERTING] %s namespace=%s name=%s error=%v", h.gvk.Kind, namespace, name, err)
		if errors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
		} else if errors.IsConflict(err) {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update status: %v", err))
		}
		return
	}

	h.logger.V(1).Info("[UPDATE-STATUS-CONVERTING] %s namespace=%s name=%s status=updated", h.gvk.Kind, namespace, name)

	// Convert to public for response
	responsePublic, _ := h.converter.PrivateToPublic(updatedPrivate)

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responsePublic)
}
