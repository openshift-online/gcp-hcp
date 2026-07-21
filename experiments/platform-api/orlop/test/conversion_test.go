package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	privatev1 "github.com/thetechnick/orlop/apis/private/test/v1"
	publicv1 "github.com/thetechnick/orlop/apis/public/test/v1"
	"github.com/thetechnick/orlop/pkg/apiserver"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	conversionTestServer        *apiserver.Server
	conversionPrivateBaseURL    string
	conversionPublicBaseURL     string
	conversionTestServerStarted bool
)

// ensureConversionTestServer starts the test server with both public and private APIs if not already started
func ensureConversionTestServer(t *testing.T) {
	if conversionTestServerStarted {
		return
	}

	// Create schemes
	privateScheme := runtime.NewScheme()
	privatev1.AddToScheme(privateScheme)

	publicScheme := runtime.NewScheme()
	publicv1.AddToScheme(publicScheme)

	// Define resources
	privateResources := []apiserver.ResourceInfo{
		{
			GVK: runtimeschema.GroupVersionKind{
				Group:   "test.orlop.thetechnick.ninja",
				Version: "v1",
				Kind:    "Object",
			},
			Plural:     privatev1.ObjectResourceInfo.Plural,
			Singular:   "object",
			Namespaced: true,
			SchemaYAML: privatev1.ObjectSchemaYAML,
		},
	}

	publicResources := []apiserver.ResourceInfo{
		{
			GVK: runtimeschema.GroupVersionKind{
				Group:   "test.orlop.thetechnick.ninja",
				Version: "v1",
				Kind:    "Object",
			},
			Plural:     publicv1.ObjectResourceInfo.Plural,
			Singular:   "object",
			Namespaced: true,
			SchemaYAML: publicv1.ObjectSchemaYAML,
		},
	}

	// Start server with both private and public APIs
	opts := apiserver.Options{
		Address:     "127.0.0.1",
		CORSOrigins: []string{"*"},
		Private: apiserver.PrivateAPIOptions{
			Port:      9003, // Different ports from other tests
			Resources: privateResources,
			Scheme:    privateScheme,
		},
		Public: apiserver.PublicAPIOptions{
			Enable:    true,
			Port:      9004,
			Resources: publicResources,
			Scheme:    publicScheme,
		},
	}

	var err error
	conversionTestServer, err = apiserver.New(opts)
	if err != nil {
		t.Fatalf("Failed to create conversion test server: %v", err)
	}

	conversionPrivateBaseURL = fmt.Sprintf("http://%s", conversionTestServer.PrivateAddress())
	conversionPublicBaseURL = fmt.Sprintf("http://%s", conversionTestServer.PublicAddress())

	// Start server in background
	go func() {
		if err := conversionTestServer.Run(); err != nil && err != http.ErrServerClosed {
			t.Logf("Conversion test server error: %v", err)
		}
	}()

	// Wait for server to be ready
	time.Sleep(200 * time.Millisecond)

	conversionTestServerStarted = true

	// Register cleanup
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conversionTestServer.Shutdown(ctx)
		conversionTestServerStarted = false
	})
}

func TestConversion_CreatePrivateReadPublic(t *testing.T) {
	ensureConversionTestServer(t)
	namespace := "default"
	name := "test-private-to-public"

	// Create object via PRIVATE API with both public and internal fields
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "public-value",
			"internalField": "internal-secret", // This should NOT appear in public API
			"nested": map[string]interface{}{
				"publicField":   "nested-public",
				"internalField": "nested-secret", // This should NOT appear in public API
			},
		},
	}

	resp, body := doConversionRequest(t, "POST", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create via private API: %d: %s", resp.StatusCode, body)
	}

	// Read via PUBLIC API
	resp, body = doConversionRequest(t, "GET", conversionPublicBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to read via public API: %d: %s", resp.StatusCode, body)
	}

	var publicObj map[string]interface{}
	if err := json.Unmarshal([]byte(body), &publicObj); err != nil {
		t.Fatalf("Failed to unmarshal public object: %v", err)
	}

	spec := publicObj["spec"].(map[string]interface{})

	// Verify public field is present
	if spec["publicField"] != "public-value" {
		t.Errorf("Expected publicField='public-value', got %v", spec["publicField"])
	}

	// Verify internal field is NOT present in public API
	if _, exists := spec["internalField"]; exists {
		t.Error("Internal field should not be exposed in public API")
	}

	nested := spec["nested"].(map[string]interface{})
	if nested["publicField"] != "nested-public" {
		t.Errorf("Expected nested.publicField='nested-public', got %v", nested["publicField"])
	}

	// Verify nested internal field is NOT present
	if _, exists := nested["internalField"]; exists {
		t.Error("Nested internal field should not be exposed in public API")
	}

	// Cleanup
	doConversionRequest(t, "DELETE", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
}

func TestConversion_CreatePublicReadPrivate(t *testing.T) {
	ensureConversionTestServer(t)
	namespace := "default"
	name := "test-public-to-private"

	// Create object via PUBLIC API (can only set public fields)
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField": "from-public-api",
			"nested": map[string]interface{}{
				"publicField": "nested-from-public",
			},
		},
	}

	resp, body := doConversionRequest(t, "POST", conversionPublicBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create via public API: %d: %s", resp.StatusCode, body)
	}

	// Read via PRIVATE API
	resp, body = doConversionRequest(t, "GET", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to read via private API: %d: %s", resp.StatusCode, body)
	}

	var privateObj map[string]interface{}
	if err := json.Unmarshal([]byte(body), &privateObj); err != nil {
		t.Fatalf("Failed to unmarshal private object: %v", err)
	}

	spec := privateObj["spec"].(map[string]interface{})

	// Verify public field is present
	if spec["publicField"] != "from-public-api" {
		t.Errorf("Expected publicField='from-public-api', got %v", spec["publicField"])
	}

	// Internal fields should exist in private API but be empty/default
	// (since they weren't set via public API)
	if internalField, exists := spec["internalField"]; exists {
		if internalField != "" && internalField != nil {
			t.Logf("Internal field has value: %v (expected empty/default)", internalField)
		}
	}

	// Cleanup
	doConversionRequest(t, "DELETE", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
}

func TestConversion_ListFromBothAPIs(t *testing.T) {
	ensureConversionTestServer(t)
	namespace := "default"

	// Create object via private API
	privateObj := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "list-test-private",
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "private-obj",
			"internalField": "secret",
			"nested": map[string]interface{}{
				"publicField":   "nested-private",
				"internalField": "nested-secret",
			},
		},
	}
	resp, body := doConversionRequest(t, "POST", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), privateObj)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create via private API: %d: %s", resp.StatusCode, body)
	}

	// Create object via public API
	publicObj := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "list-test-public",
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField": "public-obj",
			"nested": map[string]interface{}{
				"publicField": "nested-public",
			},
		},
	}
	resp, body = doConversionRequest(t, "POST", conversionPublicBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), publicObj)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create via public API: %d: %s", resp.StatusCode, body)
	}

	// List via PRIVATE API
	resp, body = doConversionRequest(t, "GET", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to list via private API: %d: %s", resp.StatusCode, body)
	}

	var privateList map[string]interface{}
	if err := json.Unmarshal([]byte(body), &privateList); err != nil {
		t.Fatalf("Failed to unmarshal private list: %v", err)
	}

	privateItems := privateList["items"].([]interface{})
	if len(privateItems) < 2 {
		t.Fatalf("Expected at least 2 items in private list, got %d", len(privateItems))
	}

	// List via PUBLIC API
	resp, body = doConversionRequest(t, "GET", conversionPublicBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to list via public API: %d: %s", resp.StatusCode, body)
	}

	var publicList map[string]interface{}
	if err := json.Unmarshal([]byte(body), &publicList); err != nil {
		t.Fatalf("Failed to unmarshal public list: %v", err)
	}

	publicItems := publicList["items"].([]interface{})
	if len(publicItems) < 2 {
		t.Fatalf("Expected at least 2 items in public list, got %d", len(publicItems))
	}

	// Verify same number of items in both APIs
	if len(privateItems) != len(publicItems) {
		t.Errorf("Expected same number of items in both APIs, private=%d public=%d", len(privateItems), len(publicItems))
	}

	// Verify public API items don't have internal fields
	for _, item := range publicItems {
		obj := item.(map[string]interface{})
		spec := obj["spec"].(map[string]interface{})
		if _, exists := spec["internalField"]; exists {
			t.Error("Public list item should not have internalField")
		}
	}

	// Cleanup
	doConversionRequest(t, "DELETE", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/list-test-private", namespace), nil)
	doConversionRequest(t, "DELETE", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/list-test-public", namespace), nil)
}

func TestConversion_UpdateViaPublicPreservesInternal(t *testing.T) {
	ensureConversionTestServer(t)
	namespace := "default"
	name := "test-update-preserve"

	// Create object via PRIVATE API with internal fields
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "initial-public",
			"internalField": "initial-secret",
			"nested": map[string]interface{}{
				"publicField":   "initial-nested-public",
				"internalField": "initial-nested-secret",
			},
		},
	}

	resp, body := doConversionRequest(t, "POST", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create: %d: %s", resp.StatusCode, body)
	}

	var created map[string]interface{}
	json.Unmarshal([]byte(body), &created)
	resourceVersion := created["metadata"].(map[string]interface{})["resourceVersion"].(string)

	// Update via PUBLIC API (can only modify public fields)
	updatePayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":            name,
			"namespace":       namespace,
			"resourceVersion": resourceVersion,
		},
		"spec": map[string]interface{}{
			"publicField": "updated-public",
			"nested": map[string]interface{}{
				"publicField": "updated-nested-public",
			},
		},
	}

	resp, body = doConversionRequest(t, "PUT", conversionPublicBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), updatePayload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to update via public API: %d: %s", resp.StatusCode, body)
	}

	// Read via PRIVATE API to verify internal fields were preserved
	resp, body = doConversionRequest(t, "GET", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to read: %d: %s", resp.StatusCode, body)
	}

	var updated map[string]interface{}
	json.Unmarshal([]byte(body), &updated)
	spec := updated["spec"].(map[string]interface{})

	// Verify public field was updated
	if spec["publicField"] != "updated-public" {
		t.Errorf("Expected publicField='updated-public', got %v", spec["publicField"])
	}

	// Verify internal field was preserved (NOT lost or cleared)
	if spec["internalField"] != "initial-secret" {
		t.Errorf("Expected internalField to be preserved as 'initial-secret', got %v", spec["internalField"])
	}

	nested := spec["nested"].(map[string]interface{})
	if nested["publicField"] != "updated-nested-public" {
		t.Errorf("Expected nested.publicField='updated-nested-public', got %v", nested["publicField"])
	}

	if nested["internalField"] != "initial-nested-secret" {
		t.Errorf("Expected nested.internalField to be preserved as 'initial-nested-secret', got %v", nested["internalField"])
	}

	// Cleanup
	doConversionRequest(t, "DELETE", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
}

func TestConversion_FilterPrivateMetadata(t *testing.T) {
	ensureConversionTestServer(t)
	namespace := "default"
	name := "test-filter-private"

	// Create object via PRIVATE API with both public and private labels/annotations
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]interface{}{
				"app":                                     "myapp",
				"private.orlop.thetechnick.ninja/secret":  "hidden",
				"private.orlop.thetechnick.ninja/owner":   "system",
				"public-label":                            "visible",
			},
			"annotations": map[string]interface{}{
				"description":                                  "public description",
				"private.orlop.thetechnick.ninja/internal-id":  "12345",
				"private.orlop.thetechnick.ninja/tracking-key": "xyz",
				"public-annotation":                            "visible",
			},
		},
		"spec": map[string]interface{}{
			"publicField":   "test",
			"internalField": "internal-value",
			"nested": map[string]interface{}{
				"publicField":   "nested-test",
				"internalField": "internal-nested-value",
			},
		},
		"status": map[string]interface{}{
			"conditions": []string{"Ready", "private.orlop.thetechnick.ninja/InternalCheck", "private.orlop.thetechnick.ninja/SecretStatus", "Available"},
		},
	}

	resp, body := doConversionRequest(t, "POST", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create via private API: %d: %s", resp.StatusCode, body)
	}

	// Read via PUBLIC API
	resp, body = doConversionRequest(t, "GET", conversionPublicBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to read via public API: %d: %s", resp.StatusCode, body)
	}

	var publicObj map[string]interface{}
	if err := json.Unmarshal([]byte(body), &publicObj); err != nil {
		t.Fatalf("Failed to unmarshal public object: %v", err)
	}

	metadata := publicObj["metadata"].(map[string]interface{})

	// Check labels - private ones should be filtered
	labels := metadata["labels"].(map[string]interface{})
	if _, exists := labels["private.orlop.thetechnick.ninja/secret"]; exists {
		t.Error("Private label 'private.orlop.thetechnick.ninja/secret' should be filtered in public API")
	}
	if _, exists := labels["private.orlop.thetechnick.ninja/owner"]; exists {
		t.Error("Private label 'private.orlop.thetechnick.ninja/owner' should be filtered in public API")
	}
	if labels["app"] != "myapp" {
		t.Errorf("Public label 'app' should be present, got %v", labels["app"])
	}
	if labels["public-label"] != "visible" {
		t.Errorf("Public label 'public-label' should be present, got %v", labels["public-label"])
	}

	// Check annotations - private ones should be filtered
	annotations := metadata["annotations"].(map[string]interface{})
	if _, exists := annotations["private.orlop.thetechnick.ninja/internal-id"]; exists {
		t.Error("Private annotation 'private.orlop.thetechnick.ninja/internal-id' should be filtered in public API")
	}
	if _, exists := annotations["private.orlop.thetechnick.ninja/tracking-key"]; exists {
		t.Error("Private annotation 'private.orlop.thetechnick.ninja/tracking-key' should be filtered in public API")
	}
	if annotations["description"] != "public description" {
		t.Errorf("Public annotation 'description' should be present, got %v", annotations["description"])
	}
	if annotations["public-annotation"] != "visible" {
		t.Errorf("Public annotation 'public-annotation' should be present, got %v", annotations["public-annotation"])
	}

	// Check conditions - private ones should be filtered
	status := publicObj["status"].(map[string]interface{})
	conditionsRaw := status["conditions"].([]interface{})

	// Convert to string slice
	conditionTypes := make([]string, 0)
	for _, c := range conditionsRaw {
		conditionTypes = append(conditionTypes, c.(string))
	}

	// Should only have Ready and Available, not the private ones
	if len(conditionTypes) != 2 {
		t.Errorf("Expected 2 conditions after filtering, got %d: %v", len(conditionTypes), conditionTypes)
	}

	hasReady := false
	hasAvailable := false
	for _, ct := range conditionTypes {
		if ct == "Ready" {
			hasReady = true
		}
		if ct == "Available" {
			hasAvailable = true
		}
		if strings.HasPrefix(ct, "private.orlop.thetechnick.ninja/") {
			t.Errorf("Private condition '%s' should be filtered in public API", ct)
		}
	}

	if !hasReady {
		t.Error("Public condition 'Ready' should be present")
	}
	if !hasAvailable {
		t.Error("Public condition 'Available' should be present")
	}

	// Read via PRIVATE API to verify original data is preserved
	resp, body = doConversionRequest(t, "GET", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to read via private API: %d: %s", resp.StatusCode, body)
	}

	var privateObj map[string]interface{}
	json.Unmarshal([]byte(body), &privateObj)

	privateMetadata := privateObj["metadata"].(map[string]interface{})
	privateLabels := privateMetadata["labels"].(map[string]interface{})
	privateAnnotations := privateMetadata["annotations"].(map[string]interface{})

	// Private API should have all labels and annotations
	if _, exists := privateLabels["private.orlop.thetechnick.ninja/secret"]; !exists {
		t.Error("Private label should be present in private API")
	}
	if _, exists := privateAnnotations["private.orlop.thetechnick.ninja/internal-id"]; !exists {
		t.Error("Private annotation should be present in private API")
	}

	privateStatus := privateObj["status"].(map[string]interface{})
	privateConditionsRaw := privateStatus["conditions"].([]interface{})
	privateConditionTypes := make([]string, 0)
	for _, c := range privateConditionsRaw {
		privateConditionTypes = append(privateConditionTypes, c.(string))
	}
	if len(privateConditionTypes) != 4 {
		t.Errorf("Expected 4 conditions in private API, got %d", len(privateConditionTypes))
	}

	// Cleanup
	doConversionRequest(t, "DELETE", conversionPrivateBaseURL+fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
}

func doConversionRequest(t *testing.T, method, url string, body interface{}) (*http.Response, string) {
	var reqBody io.Reader
	if body != nil {
		jsonData, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatal(err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	return resp, string(respBody)
}
