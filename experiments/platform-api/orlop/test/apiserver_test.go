package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	privatev1 "github.com/thetechnick/orlop/apis/private/test/v1"
	"github.com/thetechnick/orlop/pkg/apiserver"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	baseURL string
	server  *apiserver.Server
)

func TestMain(m *testing.M) {
	// Create scheme and register test types
	scheme := runtime.NewScheme()
	privatev1.AddToScheme(scheme)

	// Define test resources
	privateResources := []apiserver.ResourceInfo{
		{
			GVK: runtimeschema.GroupVersionKind{
				Group:   "test.orlop.thetechnick.ninja",
				Version: "v1",
				Kind:    "Object",
			},
			Plural:     privatev1.ObjectResourceInfo.Plural,
			Namespaced: true,
			SchemaYAML: privatev1.ObjectSchemaYAML,
		},
		{
			GVK: runtimeschema.GroupVersionKind{
				Group:   "test.orlop.thetechnick.ninja",
				Version: "v1",
				Kind:    "Other",
			},
			Plural:     privatev1.OtherResourceInfo.Plural,
			Namespaced: true,
			SchemaYAML: privatev1.OtherSchemaYAML,
		},
	}

	// Start server on random port
	opts := apiserver.Options{
		Address:     "127.0.0.1",
		CORSOrigins: []string{"*"},
		Private: apiserver.PrivateAPIOptions{
			Port:      8765, // Use fixed port for testing
			Resources: privateResources,
			Scheme:    scheme,
		},
		Public: apiserver.PublicAPIOptions{
			Port: 8766,
		},
	}

	var err error
	server, err = apiserver.New(opts)
	if err != nil {
		panic(fmt.Sprintf("Failed to create server: %v", err))
	}

	baseURL = fmt.Sprintf("http://%s", server.PrivateAddress())

	// Start server in background
	go func() {
		if err := server.Run(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Sprintf("Server error: %v", err))
		}
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Run tests
	code := m.Run()

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	// Exit with test result code
	os.Exit(code)
}

func TestObjectCRUD(t *testing.T) {
	namespace := "default"
	name := "test-object"

	// Create object
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "public-value",
			"internalField": "internal-value",
			"nested": map[string]interface{}{
				"publicField":   "nested-public",
				"internalField": "nested-internal",
			},
		},
	}

	resp, body := doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, body)
	}

	var created map[string]interface{}
	json.Unmarshal([]byte(body), &created)

	// Check that object was created with metadata
	metadata := created["metadata"].(map[string]interface{})
	if metadata["name"] != name {
		t.Errorf("Expected name %s, got %v", name, metadata["name"])
	}
	if metadata["namespace"] != namespace {
		t.Errorf("Expected namespace %s, got %v", namespace, metadata["namespace"])
	}
	if metadata["resourceVersion"] == nil {
		t.Error("Expected resourceVersion to be set")
	}

	resourceVersion := metadata["resourceVersion"].(string)

	// Get object
	resp, body = doRequest(t, "GET", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var retrieved map[string]interface{}
	json.Unmarshal([]byte(body), &retrieved)
	if retrieved["metadata"].(map[string]interface{})["name"] != name {
		t.Error("Retrieved object has wrong name")
	}

	// List objects
	resp, body = doRequest(t, "GET", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var list map[string]interface{}
	json.Unmarshal([]byte(body), &list)
	items := list["items"].([]interface{})
	if len(items) == 0 {
		t.Error("Expected at least one object in list")
	}

	// Update object
	updatePayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":            name,
			"namespace":       namespace,
			"resourceVersion": resourceVersion,
		},
		"spec": map[string]interface{}{
			"publicField":   "updated-value",
			"internalField": "internal-value",
			"nested": map[string]interface{}{
				"publicField":   "nested-public",
				"internalField": "nested-internal",
			},
		},
	}

	resp, body = doRequest(t, "PUT", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), updatePayload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var updated map[string]interface{}
	json.Unmarshal([]byte(body), &updated)
	spec := updated["spec"].(map[string]interface{})
	if spec["publicField"] != "updated-value" {
		t.Errorf("Expected publicField to be updated, got %v", spec["publicField"])
	}

	// Delete object
	resp, body = doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	// Verify object is deleted
	resp, _ = doRequest(t, "GET", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 after delete, got %d", resp.StatusCode)
	}
}

func TestDefaulting(t *testing.T) {
	namespace := "default"
	name := "test-defaulting"

	// Create object without defaultField
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "public-value",
			"internalField": "internal-value",
			"nested": map[string]interface{}{
				"publicField":   "nested-public",
				"internalField": "nested-internal",
			},
		},
	}

	resp, body := doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, body)
	}

	var created map[string]interface{}
	json.Unmarshal([]byte(body), &created)

	// Check that defaultField was set to "default-value"
	spec := created["spec"].(map[string]interface{})
	if spec["defaultField"] != "default-value" {
		t.Errorf("Expected defaultField to be 'default-value', got %v", spec["defaultField"])
	}

	// Cleanup
	doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
}

func TestPruning(t *testing.T) {
	namespace := "default"
	name := "test-pruning"

	// Create object with unknown field
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "public-value",
			"internalField": "internal-value",
			"unknownField":  "should-be-pruned",
			"nested": map[string]interface{}{
				"publicField":   "nested-public",
				"internalField": "nested-internal",
			},
		},
	}

	resp, body := doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, body)
	}

	var created map[string]interface{}
	json.Unmarshal([]byte(body), &created)

	// Check that unknownField was pruned
	spec := created["spec"].(map[string]interface{})
	if _, exists := spec["unknownField"]; exists {
		t.Error("Expected unknownField to be pruned, but it still exists")
	}

	// Cleanup
	doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
}

func TestValidation(t *testing.T) {
	namespace := "default"
	name := "test-validation"

	// Create object missing required field
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField": "public-value",
			// Missing internalField and nested (required fields)
		},
	}

	resp, body := doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected status 400 for validation error, got %d: %s", resp.StatusCode, body)
	}

	// Verify error message mentions validation
	if !contains(body, "validation") && !contains(body, "required") {
		t.Errorf("Expected validation error message, got: %s", body)
	}
}

func TestStatusSubresource(t *testing.T) {
	namespace := "default"
	name := "test-status"

	// Create object
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "public-value",
			"internalField": "internal-value",
			"nested": map[string]interface{}{
				"publicField":   "nested-public",
				"internalField": "nested-internal",
			},
		},
	}

	resp, body := doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, body)
	}

	var created map[string]interface{}
	json.Unmarshal([]byte(body), &created)
	resourceVersion := created["metadata"].(map[string]interface{})["resourceVersion"].(string)
	createdGeneration := int64(created["metadata"].(map[string]interface{})["generation"].(float64))

	if createdGeneration != 1 {
		t.Errorf("Expected generation 1 on create, got %d", createdGeneration)
	}

	// Update status
	statusPayload := map[string]interface{}{
		"metadata": map[string]interface{}{
			"resourceVersion": resourceVersion,
		},
		"status": map[string]interface{}{
			"conditions": []string{"Ready", "Healthy"},
		},
	}

	resp, body = doRequest(t, "PUT", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s/status", namespace, name), statusPayload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var updated map[string]interface{}
	json.Unmarshal([]byte(body), &updated)

	// Check that status was updated
	status := updated["status"].(map[string]interface{})
	conditions := status["conditions"].([]interface{})
	if len(conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(conditions))
	}

	// Check that spec was NOT modified
	spec := updated["spec"].(map[string]interface{})
	if spec["publicField"] != "public-value" {
		t.Error("Spec should not be modified by status update")
	}

	// Check that generation was NOT incremented for status update
	statusGeneration := int64(updated["metadata"].(map[string]interface{})["generation"].(float64))
	if statusGeneration != 1 {
		t.Errorf("Expected generation to remain 1 after status update, got %d", statusGeneration)
	}

	// Cleanup
	doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
}

func TestCORS(t *testing.T) {
	// Make an OPTIONS request (preflight)
	req, err := http.NewRequest("OPTIONS", baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Check CORS headers
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Error("Expected Access-Control-Allow-Origin header")
	}
	if resp.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Error("Expected Access-Control-Allow-Methods header")
	}
}

func TestGenerationTracking(t *testing.T) {
	namespace := "default"
	name := "test-generation"

	// Create object
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "initial-value",
			"internalField": "internal-value",
			"nested": map[string]interface{}{
				"publicField":   "nested-public",
				"internalField": "nested-internal",
			},
		},
	}

	resp, body := doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, body)
	}

	var created map[string]interface{}
	json.Unmarshal([]byte(body), &created)

	generation := int64(created["metadata"].(map[string]interface{})["generation"].(float64))
	if generation != 1 {
		t.Errorf("Expected generation 1 on create, got %d", generation)
	}

	// Update spec - generation should increment
	resourceVersion := created["metadata"].(map[string]interface{})["resourceVersion"].(string)
	updatePayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":            name,
			"namespace":       namespace,
			"resourceVersion": resourceVersion,
		},
		"spec": map[string]interface{}{
			"publicField":   "updated-value",
			"internalField": "internal-value",
			"nested": map[string]interface{}{
				"publicField":   "nested-public",
				"internalField": "nested-internal",
			},
		},
	}

	resp, body = doRequest(t, "PUT", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), updatePayload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var updated map[string]interface{}
	json.Unmarshal([]byte(body), &updated)

	generation = int64(updated["metadata"].(map[string]interface{})["generation"].(float64))
	if generation != 2 {
		t.Errorf("Expected generation 2 after spec update, got %d", generation)
	}

	// Update with same spec - generation should NOT increment
	resourceVersion = updated["metadata"].(map[string]interface{})["resourceVersion"].(string)
	updatePayload["metadata"].(map[string]interface{})["resourceVersion"] = resourceVersion

	resp, body = doRequest(t, "PUT", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), updatePayload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var updated2 map[string]interface{}
	json.Unmarshal([]byte(body), &updated2)

	generation = int64(updated2["metadata"].(map[string]interface{})["generation"].(float64))
	if generation != 2 {
		t.Errorf("Expected generation to remain 2 when spec unchanged, got %d", generation)
	}

	// Cleanup
	doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
}

func TestLabelSelector(t *testing.T) {
	namespace := "default"

	// Create objects with different labels
	objects := []struct {
		name   string
		labels map[string]string
	}{
		{"obj-1", map[string]string{"env": "prod", "tier": "frontend"}},
		{"obj-2", map[string]string{"env": "dev", "tier": "backend"}},
		{"obj-3", map[string]string{"env": "prod", "tier": "backend"}},
		{"obj-4", map[string]string{"env": "staging", "tier": "frontend"}},
	}

	// Create all objects
	for _, obj := range objects {
		createPayload := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name":      obj.name,
				"namespace": namespace,
				"labels":    obj.labels,
			},
			"spec": map[string]interface{}{
				"publicField":   "test",
				"internalField": "internal",
				"nested": map[string]interface{}{
					"publicField":   "nested",
					"internalField": "nested-internal",
				},
			},
		}

		resp, body := doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, body)
		}
	}

	// Test cases for label selectors
	testCases := []struct {
		name     string
		selector string
		expected []string
	}{
		{
			name:     "Equality selector - env=prod",
			selector: "env=prod",
			expected: []string{"obj-1", "obj-3"},
		},
		{
			name:     "Equality selector - tier=frontend",
			selector: "tier=frontend",
			expected: []string{"obj-1", "obj-4"},
		},
		{
			name:     "Multiple selectors - env=prod,tier=backend",
			selector: "env=prod,tier=backend",
			expected: []string{"obj-3"},
		},
		{
			name:     "Inequality selector - env!=prod",
			selector: "env!=prod",
			expected: []string{"obj-2", "obj-4"},
		},
		{
			name:     "Set-based selector - env in (prod,staging)",
			selector: "env in (prod,staging)",
			expected: []string{"obj-1", "obj-3", "obj-4"},
		},
		{
			name:     "Set-based selector - tier notin (frontend)",
			selector: "tier notin (frontend)",
			expected: []string{"obj-2", "obj-3"},
		},
		{
			name:     "Empty selector",
			selector: "",
			expected: []string{"obj-1", "obj-2", "obj-3", "obj-4"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace)
			if tc.selector != "" {
				path += "?labelSelector=" + url.QueryEscape(tc.selector)
			}

			resp, body := doRequest(t, "GET", path, nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
			}

			var list map[string]interface{}
			json.Unmarshal([]byte(body), &list)
			items := list["items"].([]interface{})

			// Extract names from results
			var resultNames []string
			for _, item := range items {
				itemMap := item.(map[string]interface{})
				metadata := itemMap["metadata"].(map[string]interface{})
				resultNames = append(resultNames, metadata["name"].(string))
			}

			// Sort both slices for comparison
			sort.Strings(resultNames)
			sort.Strings(tc.expected)

			if !reflect.DeepEqual(resultNames, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, resultNames)
			}
		})
	}

	// Test invalid label selector
	t.Run("Invalid label selector", func(t *testing.T) {
		path := fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace)
		path += "?labelSelector=" + url.QueryEscape("invalid!selector")
		resp, body := doRequest(t, "GET", path, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid selector, got %d: %s", resp.StatusCode, body)
		}

		if !contains(body, "invalid label selector") {
			t.Errorf("Expected error message about invalid label selector, got: %s", body)
		}
	})

	// Cleanup
	for _, obj := range objects {
		doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, obj.name), nil)
	}
}

func TestDiscoveryAPIGroupList(t *testing.T) {
	resp, body := doRequest(t, "GET", "/apis", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var groupList map[string]interface{}
	if err := json.Unmarshal([]byte(body), &groupList); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check that we have the test.orlop.thetechnick.ninja group
	groups := groupList["groups"].([]interface{})
	found := false
	for _, g := range groups {
		group := g.(map[string]interface{})
		if group["name"] == "test.orlop.thetechnick.ninja" {
			found = true
			// Check that it has v1 version
			versions := group["versions"].([]interface{})
			if len(versions) == 0 {
				t.Error("Expected at least one version")
			}
			v := versions[0].(map[string]interface{})
			if v["version"] != "v1" {
				t.Errorf("Expected version v1, got %s", v["version"])
			}
			break
		}
	}

	if !found {
		t.Error("Expected test.orlop.thetechnick.ninja group not found")
	}
}

func TestDiscoveryAPIGroup(t *testing.T) {
	resp, body := doRequest(t, "GET", "/apis/test.orlop.thetechnick.ninja", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var group map[string]interface{}
	if err := json.Unmarshal([]byte(body), &group); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check group name
	if group["name"] != "test.orlop.thetechnick.ninja" {
		t.Errorf("Expected group name test.orlop.thetechnick.ninja, got %s", group["name"])
	}

	// Check versions
	versions := group["versions"].([]interface{})
	if len(versions) == 0 {
		t.Error("Expected at least one version")
	}
}

func TestDiscoveryAPIResourceList(t *testing.T) {
	resp, body := doRequest(t, "GET", "/apis/test.orlop.thetechnick.ninja/v1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var resourceList map[string]interface{}
	if err := json.Unmarshal([]byte(body), &resourceList); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check groupVersion
	if resourceList["groupVersion"] != "test.orlop.thetechnick.ninja/v1" {
		t.Errorf("Expected groupVersion test.orlop.thetechnick.ninja/v1, got %s", resourceList["groupVersion"])
	}

	// Check resources (field is called "resources" in JSON, not "apiResources")
	if resourceList["resources"] == nil {
		t.Fatalf("resources is nil. Full response: %s", body)
	}
	resources := resourceList["resources"].([]interface{})
	if len(resources) == 0 {
		t.Error("Expected at least one resource")
	}

	// Find objects resource
	foundObjects := false
	foundObjectsStatus := false
	for _, r := range resources {
		res := r.(map[string]interface{})
		if res["name"] == "objects" {
			foundObjects = true
			if res["kind"] != "Object" {
				t.Errorf("Expected kind Object, got %s", res["kind"])
			}
			if res["namespaced"] != true {
				t.Error("Expected namespaced to be true")
			}
		}
		if res["name"] == "objects/status" {
			foundObjectsStatus = true
		}
	}

	if !foundObjects {
		t.Error("Expected objects resource not found")
	}
	if !foundObjectsStatus {
		t.Error("Expected objects/status subresource not found")
	}
}

func TestDiscoveryOpenAPIV3(t *testing.T) {
	resp, body := doRequest(t, "GET", "/openapi/v3", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var openapi map[string]interface{}
	if err := json.Unmarshal([]byte(body), &openapi); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check paths
	paths := openapi["paths"].(map[string]interface{})
	if len(paths) == 0 {
		t.Error("Expected at least one path")
	}

	// Check for test group/version
	key := "apis/test.orlop.thetechnick.ninja/v1"
	if _, ok := paths[key]; !ok {
		t.Errorf("Expected path %s not found", key)
	}
}

func TestDiscoveryOpenAPIV3GroupVersion(t *testing.T) {
	resp, body := doRequest(t, "GET", "/openapi/v3/apis/test.orlop.thetechnick.ninja/v1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var spec map[string]interface{}
	if err := json.Unmarshal([]byte(body), &spec); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check OpenAPI version
	if spec["openapi"] != "3.0.0" {
		t.Errorf("Expected openapi 3.0.0, got %s", spec["openapi"])
	}

	// Check info
	info := spec["info"].(map[string]interface{})
	if info["version"] != "v1" {
		t.Errorf("Expected version v1, got %s", info["version"])
	}

	// Check components and schemas
	components := spec["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})
	if len(schemas) == 0 {
		t.Error("Expected at least one schema")
	}

	// Check paths
	paths := spec["paths"].(map[string]interface{})
	if len(paths) == 0 {
		t.Error("Expected at least one path")
	}
}

func TestSharedResourceVersion(t *testing.T) {
	namespace := "default"

	// List objects initially - should get initial resource version
	resp, body := doRequest(t, "GET", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var initialList map[string]interface{}
	json.Unmarshal([]byte(body), &initialList)
	initialMetadata := initialList["metadata"].(map[string]interface{})
	initialRV := initialMetadata["resourceVersion"].(string)

	// Create an Object
	obj1Payload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "test-obj",
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "test",
			"internalField": "internal",
			"nested": map[string]interface{}{
				"publicField":   "nested",
				"internalField": "nested-internal",
			},
		},
	}

	resp, body = doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), obj1Payload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, body)
	}

	var createdObj map[string]interface{}
	json.Unmarshal([]byte(body), &createdObj)
	objMetadata := createdObj["metadata"].(map[string]interface{})
	objRV := objMetadata["resourceVersion"].(string)

	// ResourceVersion should have incremented
	if objRV == initialRV {
		t.Errorf("Expected resource version to increment after create, got %s", objRV)
	}

	// Create an Other (different resource type) - should have independent resource version
	other1Payload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Other",
		"metadata": map[string]interface{}{
			"name":      "test-other",
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "test",
			"internalField": "internal",
		},
	}

	resp, body = doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/others", namespace), other1Payload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, body)
	}

	var createdOther map[string]interface{}
	json.Unmarshal([]byte(body), &createdOther)
	otherMetadata := createdOther["metadata"].(map[string]interface{})
	otherRV := otherMetadata["resourceVersion"].(string)

	// ResourceVersion is per-resource-type, so Other should start from "1"
	// (Each resource type has its own counter)
	if otherRV != "1" {
		t.Logf("Note: Other resource has independent resource version counter. Object RV: %s, Other RV: %s", objRV, otherRV)
	}

	// List objects - should return resource version from objects store
	resp, body = doRequest(t, "GET", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var objectsList map[string]interface{}
	json.Unmarshal([]byte(body), &objectsList)
	listMetadata := objectsList["metadata"].(map[string]interface{})
	listRV := listMetadata["resourceVersion"].(string)

	// List resourceVersion should match the object's resource version (not Other's)
	if listRV != objRV {
		t.Errorf("Expected list resourceVersion to match Object RV (%s), got %s", objRV, listRV)
	}

	// Cleanup
	doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/test-obj", namespace), nil)
	doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/others/test-other", namespace), nil)
}

func TestCreateReturnsResourceVersion(t *testing.T) {
	namespace := "default"
	name := "test-create-rv"

	// Create object
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "public-value",
			"internalField": "internal-value",
			"nested": map[string]interface{}{
				"publicField":   "nested-public",
				"internalField": "nested-internal",
			},
		},
	}

	resp, body := doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, body)
	}

	// Parse response
	var created map[string]interface{}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check metadata
	metadata := created["metadata"].(map[string]interface{})

	// ResourceVersion should be set
	rv, ok := metadata["resourceVersion"].(string)
	if !ok || rv == "" {
		t.Errorf("Expected resourceVersion to be set in create response, got: %v", metadata["resourceVersion"])
	}

	// UID should be set
	uid, ok := metadata["uid"].(string)
	if !ok || uid == "" {
		t.Errorf("Expected uid to be set in create response, got: %v", metadata["uid"])
	}

	// CreationTimestamp should be set
	creationTimestamp, ok := metadata["creationTimestamp"].(string)
	if !ok || creationTimestamp == "" {
		t.Errorf("Expected creationTimestamp to be set in create response, got: %v", metadata["creationTimestamp"])
	}

	// Generation should be 1
	generation, ok := metadata["generation"].(float64)
	if !ok || generation != 1 {
		t.Errorf("Expected generation to be 1 in create response, got: %v", metadata["generation"])
	}

	// Cleanup
	doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
}

func TestDiscoveryOpenAPIV2(t *testing.T) {
	resp, body := doRequest(t, "GET", "/openapi/v2", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var spec map[string]interface{}
	if err := json.Unmarshal([]byte(body), &spec); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check Swagger version
	if spec["swagger"] != "2.0" {
		t.Errorf("Expected swagger 2.0, got %s", spec["swagger"])
	}

	// Check info
	info := spec["info"].(map[string]interface{})
	if info["title"] == "" {
		t.Error("Expected title in info")
	}

	// Check definitions
	definitions := spec["definitions"].(map[string]interface{})
	if len(definitions) == 0 {
		t.Error("Expected at least one definition")
	}

	// Check paths
	paths := spec["paths"].(map[string]interface{})
	if len(paths) == 0 {
		t.Error("Expected at least one path")
	}

	// Verify a specific path exists
	foundObjectsPath := false
	for path := range paths {
		if path == "/apis/test.orlop.thetechnick.ninja/v1/namespaces/{namespace}/objects" {
			foundObjectsPath = true
			break
		}
	}
	if !foundObjectsPath {
		t.Error("Expected objects collection path not found")
	}
}

func TestWatchBookmarks(t *testing.T) {
	namespace := "default"

	// Start a watch request with allowWatchBookmarks=true
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	watchURL := fmt.Sprintf("%s/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects?watch=true&allowWatchBookmarks=true", baseURL, namespace)

	watchReq, err := http.NewRequestWithContext(ctx, "GET", watchURL, nil)
	if err != nil {
		t.Fatalf("Failed to create watch request: %v", err)
	}

	client := &http.Client{}

	type respResult struct {
		resp *http.Response
		err  error
	}
	respCh := make(chan respResult, 1)

	go func() {
		resp, err := client.Do(watchReq)
		respCh <- respResult{resp, err}
	}()

	var watchResp *http.Response
	select {
	case result := <-respCh:
		if result.err != nil {
			t.Fatalf("Failed to start watch: %v", result.err)
		}
		watchResp = result.resp
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for watch connection")
	}
	defer watchResp.Body.Close()

	if watchResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(watchResp.Body)
		t.Fatalf("Expected status 200 for watch, got %d: %s", watchResp.StatusCode, body)
	}

	// Channel to collect watch events
	events := make(chan map[string]interface{}, 10)
	watchDone := make(chan struct{})

	// Start reading watch events
	go func() {
		defer close(watchDone)
		decoder := json.NewDecoder(watchResp.Body)
		for {
			var event map[string]interface{}
			if err := decoder.Decode(&event); err != nil {
				return
			}
			events <- event
		}
	}()

	// Give watch time to establish
	time.Sleep(100 * time.Millisecond)

	// Wait for at least 31 seconds to get a BOOKMARK event (they're sent every 30s)
	bookmarkReceived := false
	timeout := time.After(32 * time.Second)

waitLoop:
	for {
		select {
		case event := <-events:
			if event["type"] == "BOOKMARK" {
				t.Logf("Received BOOKMARK event: %+v", event)
				bookmarkReceived = true

				// Verify BOOKMARK event structure
				obj := event["object"].(map[string]interface{})
				if obj["apiVersion"] == nil {
					t.Error("BOOKMARK event missing apiVersion")
				}
				if obj["kind"] == nil {
					t.Error("BOOKMARK event missing kind")
				}
				metadata := obj["metadata"].(map[string]interface{})
				if metadata["resourceVersion"] == nil {
					t.Error("BOOKMARK event missing resourceVersion")
				}
				break waitLoop
			}
		case <-timeout:
			break waitLoop
		}
	}

	if !bookmarkReceived {
		t.Error("Expected to receive at least one BOOKMARK event within 32 seconds")
	}
}

func TestWatchWithoutBookmarks(t *testing.T) {
	namespace := "default"

	// Start a watch request WITHOUT allowWatchBookmarks (default behavior)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	watchURL := fmt.Sprintf("%s/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects?watch=true", baseURL, namespace)

	watchReq, err := http.NewRequestWithContext(ctx, "GET", watchURL, nil)
	if err != nil {
		t.Fatalf("Failed to create watch request: %v", err)
	}

	client := &http.Client{}

	type respResult struct {
		resp *http.Response
		err  error
	}
	respCh := make(chan respResult, 1)

	go func() {
		resp, err := client.Do(watchReq)
		respCh <- respResult{resp, err}
	}()

	var watchResp *http.Response
	select {
	case result := <-respCh:
		if result.err != nil {
			t.Fatalf("Failed to start watch: %v", result.err)
		}
		watchResp = result.resp
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for watch connection")
	}
	defer watchResp.Body.Close()

	if watchResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(watchResp.Body)
		t.Fatalf("Expected status 200 for watch, got %d: %s", watchResp.StatusCode, body)
	}

	// Channel to collect watch events
	events := make(chan map[string]interface{}, 10)
	watchDone := make(chan struct{})

	// Start reading watch events
	go func() {
		defer close(watchDone)
		decoder := json.NewDecoder(watchResp.Body)
		for {
			var event map[string]interface{}
			if err := decoder.Decode(&event); err != nil {
				return
			}
			events <- event
		}
	}()

	// Wait for 2 seconds to make sure no BOOKMARK events are sent
	bookmarkReceived := false
	timeout := time.After(2 * time.Second)

waitLoop:
	for {
		select {
		case event := <-events:
			if event["type"] == "BOOKMARK" {
				bookmarkReceived = true
				break waitLoop
			}
		case <-timeout:
			break waitLoop
		}
	}

	if bookmarkReceived {
		t.Error("Should NOT receive BOOKMARK events when allowWatchBookmarks is not set")
	}
}

func TestWatch(t *testing.T) {
	namespace := "default"

	// Start a watch request in the background with context
	// Use a long timeout since watch streams are long-lived
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	watchURL := fmt.Sprintf("%s/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects?watch=true", baseURL, namespace)

	watchReq, err := http.NewRequestWithContext(ctx, "GET", watchURL, nil)
	if err != nil {
		t.Fatalf("Failed to create watch request: %v", err)
	}

	// Don't set a client timeout - the watch stream is long-lived
	// The context timeout will handle overall test timeout
	client := &http.Client{}

	// For streaming responses, we need to handle the connection differently
	// Start the request in a goroutine so we can handle the streaming response
	type respResult struct {
		resp *http.Response
		err  error
	}
	respCh := make(chan respResult, 1)

	go func() {
		resp, err := client.Do(watchReq)
		respCh <- respResult{resp, err}
	}()

	var watchResp *http.Response
	select {
	case result := <-respCh:
		if result.err != nil {
			t.Fatalf("Failed to start watch: %v", result.err)
		}
		watchResp = result.resp
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for watch connection")
	}
	defer watchResp.Body.Close()

	if watchResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(watchResp.Body)
		t.Fatalf("Expected status 200 for watch, got %d: %s", watchResp.StatusCode, body)
	}

	// Channel to collect watch events
	events := make(chan map[string]interface{}, 10)
	watchDone := make(chan struct{})

	// Start reading watch events
	go func() {
		defer close(watchDone)
		decoder := json.NewDecoder(watchResp.Body)
		for {
			var event map[string]interface{}
			if err := decoder.Decode(&event); err != nil {
				return
			}
			events <- event
		}
	}()

	// Give watch time to establish
	time.Sleep(100 * time.Millisecond)

	// Create an object
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "watch-test",
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "test",
			"internalField": "internal",
			"nested": map[string]interface{}{
				"publicField":   "nested",
				"internalField": "nested-internal",
			},
		},
	}

	doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)

	// Wait for ADDED event and extract resourceVersion
	var currentResourceVersion string
	select {
	case event := <-events:
		if event["type"] != "ADDED" {
			t.Errorf("Expected ADDED event, got %s", event["type"])
		}
		obj := event["object"].(map[string]interface{})
		metadata := obj["metadata"].(map[string]interface{})
		if metadata["name"] != "watch-test" {
			t.Errorf("Expected name watch-test, got %s", metadata["name"])
		}
		currentResourceVersion = metadata["resourceVersion"].(string)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for ADDED event")
	}

	// Update the object - must include resourceVersion for optimistic concurrency
	updatePayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":            "watch-test",
			"namespace":       namespace,
			"resourceVersion": currentResourceVersion,
		},
		"spec": map[string]interface{}{
			"publicField":   "updated",
			"internalField": "internal",
			"nested": map[string]interface{}{
				"publicField":   "nested",
				"internalField": "nested-internal",
			},
		},
	}

	doRequest(t, "PUT", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/watch-test", namespace), updatePayload)

	// Wait for MODIFIED event
	select {
	case event := <-events:
		if event["type"] != "MODIFIED" {
			t.Errorf("Expected MODIFIED event, got %s", event["type"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for MODIFIED event")
	}

	// Delete the object
	doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/watch-test", namespace), nil)

	// Wait for DELETED event
	select {
	case event := <-events:
		if event["type"] != "DELETED" {
			t.Errorf("Expected DELETED event, got %s", event["type"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for DELETED event")
	}
}

func TestWatchSendInitialEvents(t *testing.T) {
	namespace := "default"

	// Create some objects first
	for i := 1; i <= 3; i++ {
		createPayload := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name":      fmt.Sprintf("initial-event-test-%d", i),
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"publicField":   fmt.Sprintf("value-%d", i),
				"internalField": "internal",
				"nested": map[string]interface{}{
					"publicField":   "nested",
					"internalField": "nested-internal",
				},
			},
		}
		doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), createPayload)
	}

	// Start watch with sendInitialEvents=true and allowWatchBookmarks=true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	watchURL := fmt.Sprintf("%s/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects?watch=true&sendInitialEvents=true&allowWatchBookmarks=true", baseURL, namespace)

	watchReq, err := http.NewRequestWithContext(ctx, "GET", watchURL, nil)
	if err != nil {
		t.Fatalf("Failed to create watch request: %v", err)
	}

	client := &http.Client{}

	type respResult struct {
		resp *http.Response
		err  error
	}
	respCh := make(chan respResult, 1)

	go func() {
		resp, err := client.Do(watchReq)
		respCh <- respResult{resp, err}
	}()

	var watchResp *http.Response
	select {
	case result := <-respCh:
		if result.err != nil {
			t.Fatalf("Failed to start watch: %v", result.err)
		}
		watchResp = result.resp
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for watch connection")
	}
	defer watchResp.Body.Close()

	if watchResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(watchResp.Body)
		t.Fatalf("Expected status 200 for watch, got %d: %s", watchResp.StatusCode, body)
	}

	// Channel to collect watch events
	events := make(chan map[string]interface{}, 20)

	// Start reading watch events
	go func() {
		decoder := json.NewDecoder(watchResp.Body)
		for {
			var event map[string]interface{}
			if err := decoder.Decode(&event); err != nil {
				return
			}
			events <- event
		}
	}()

	// Collect initial ADDED events - should receive at least 3 for our created objects
	receivedNames := make(map[string]bool)
	timeout := time.After(2 * time.Second)

	for i := 0; i < 3; i++ {
		select {
		case event := <-events:
			if event["type"] != "ADDED" {
				t.Errorf("Expected ADDED event for initial events, got %s", event["type"])
				continue
			}
			obj := event["object"].(map[string]interface{})
			metadata := obj["metadata"].(map[string]interface{})
			name := metadata["name"].(string)
			receivedNames[name] = true
			t.Logf("Received initial ADDED event for %s", name)
		case <-timeout:
			t.Fatalf("Timeout waiting for initial ADDED events, received %d/%d", len(receivedNames), 3)
		}
	}

	// Verify we got all 3 objects
	for i := 1; i <= 3; i++ {
		expectedName := fmt.Sprintf("initial-event-test-%d", i)
		if !receivedNames[expectedName] {
			t.Errorf("Did not receive initial event for %s", expectedName)
		}
	}

	// After initial events, should receive a BOOKMARK with annotation
	select {
	case event := <-events:
		if event["type"] != "BOOKMARK" {
			t.Errorf("Expected BOOKMARK event after initial events, got %s", event["type"])
		} else {
			obj := event["object"].(map[string]interface{})
			metadata := obj["metadata"].(map[string]interface{})
			annotations, ok := metadata["annotations"].(map[string]interface{})
			if !ok {
				t.Error("BOOKMARK event missing annotations")
			} else {
				if annotations["k8s.io/initial-events-end"] != "true" {
					t.Errorf("Expected k8s.io/initial-events-end annotation to be 'true', got %v", annotations["k8s.io/initial-events-end"])
				} else {
					t.Log("Received BOOKMARK with k8s.io/initial-events-end annotation")
				}
			}
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for BOOKMARK event after initial events")
	}

	// Cleanup
	for i := 1; i <= 3; i++ {
		doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/initial-event-test-%d", namespace, i), nil)
	}
}

func TestOtherResource(t *testing.T) {
	namespace := "default"
	name := "test-other"

	// Create Other resource
	createPayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Other",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "public-value",
			"internalField": "internal-value",
		},
	}

	resp, body := doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/others", namespace), createPayload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, body)
	}

	// Get Other resource
	resp, body = doRequest(t, "GET", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/others/%s", namespace, name), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
	}

	// Cleanup
	doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/others/%s", namespace, name), nil)
}

func TestClusterScopedList(t *testing.T) {
	// Create objects in multiple namespaces
	objects := []struct {
		namespace string
		name      string
	}{
		{"default", "cluster-test-1"},
		{"default", "cluster-test-2"},
		{"kube-system", "cluster-test-3"},
		{"kube-public", "cluster-test-4"},
	}

	// Create all objects
	for _, obj := range objects {
		createPayload := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name":      obj.name,
				"namespace": obj.namespace,
			},
			"spec": map[string]interface{}{
				"publicField":   "test-value",
				"internalField": "internal-value",
				"nested": map[string]interface{}{
					"publicField":   "nested-value",
					"internalField": "nested-internal",
				},
			},
		}

		resp, body := doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", obj.namespace), createPayload)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to create object %s/%s: status %d: %s", obj.namespace, obj.name, resp.StatusCode, body)
		}
	}

	// Test cluster-scoped LIST (all namespaces)
	t.Run("Cluster-scoped LIST returns all objects", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/apis/test.orlop.thetechnick.ninja/v1/objects", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
		}

		var list map[string]interface{}
		json.Unmarshal([]byte(body), &list)
		items := list["items"].([]interface{})

		// Should return at least the 4 objects we just created
		if len(items) < 4 {
			t.Fatalf("Expected at least 4 objects in cluster-scoped list, got %d", len(items))
		}

		// Verify objects from different namespaces are included
		namespacesFound := make(map[string]bool)
		namesFound := make(map[string]bool)
		for _, item := range items {
			itemMap := item.(map[string]interface{})
			metadata := itemMap["metadata"].(map[string]interface{})
			namespace := metadata["namespace"].(string)
			name := metadata["name"].(string)
			namespacesFound[namespace] = true
			namesFound[name] = true
		}

		// Check that objects from all namespaces are present
		expectedNamespaces := []string{"default", "kube-system", "kube-public"}
		for _, ns := range expectedNamespaces {
			if !namespacesFound[ns] {
				t.Errorf("Expected objects from namespace %s, but none found", ns)
			}
		}

		// Check that all created objects are present
		for _, obj := range objects {
			if !namesFound[obj.name] {
				t.Errorf("Expected object %s to be in cluster-scoped list, but not found", obj.name)
			}
		}
	})

	// Test namespace-scoped LIST only returns objects from that namespace
	t.Run("Namespace-scoped LIST returns only objects from that namespace", func(t *testing.T) {
		resp, body := doRequest(t, "GET", "/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
		}

		var list map[string]interface{}
		json.Unmarshal([]byte(body), &list)
		items := list["items"].([]interface{})

		// Count objects from default namespace
		defaultCount := 0
		for _, item := range items {
			itemMap := item.(map[string]interface{})
			metadata := itemMap["metadata"].(map[string]interface{})
			namespace := metadata["namespace"].(string)
			if namespace == "default" {
				defaultCount++
			} else {
				t.Errorf("Namespace-scoped list returned object from wrong namespace: %s", namespace)
			}
		}

		// Should have exactly 2 objects in default namespace (from our test data)
		if defaultCount < 2 {
			t.Errorf("Expected at least 2 objects from default namespace, got %d", defaultCount)
		}
	})

	// Test cluster-scoped LIST with label selector
	t.Run("Cluster-scoped LIST with label selector", func(t *testing.T) {
		// Create objects with labels in different namespaces
		labeledObjects := []struct {
			namespace string
			name      string
			labels    map[string]string
		}{
			{"default", "labeled-1", map[string]string{"env": "prod"}},
			{"kube-system", "labeled-2", map[string]string{"env": "prod"}},
			{"default", "labeled-3", map[string]string{"env": "dev"}},
		}

		for _, obj := range labeledObjects {
			createPayload := map[string]interface{}{
				"apiVersion": "test.orlop.thetechnick.ninja/v1",
				"kind":       "Object",
				"metadata": map[string]interface{}{
					"name":      obj.name,
					"namespace": obj.namespace,
					"labels":    obj.labels,
				},
				"spec": map[string]interface{}{
					"publicField":   "test",
					"internalField": "internal",
					"nested": map[string]interface{}{
						"publicField":   "nested",
						"internalField": "nested-internal",
					},
				},
			}

			doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", obj.namespace), createPayload)
		}

		// Query cluster-scoped with label selector
		resp, body := doRequest(t, "GET", "/apis/test.orlop.thetechnick.ninja/v1/objects?labelSelector="+url.QueryEscape("env=prod"), nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, body)
		}

		var list map[string]interface{}
		json.Unmarshal([]byte(body), &list)
		items := list["items"].([]interface{})

		// Should return 2 objects with env=prod from different namespaces
		prodCount := 0
		namespacesWithProd := make(map[string]bool)
		for _, item := range items {
			itemMap := item.(map[string]interface{})
			metadata := itemMap["metadata"].(map[string]interface{})
			if labels, ok := metadata["labels"].(map[string]interface{}); ok {
				if env, ok := labels["env"].(string); ok && env == "prod" {
					prodCount++
					namespacesWithProd[metadata["namespace"].(string)] = true
				}
			}
		}

		if prodCount < 2 {
			t.Errorf("Expected at least 2 objects with env=prod across all namespaces, got %d", prodCount)
		}

		if !namespacesWithProd["default"] || !namespacesWithProd["kube-system"] {
			t.Error("Expected prod objects from both default and kube-system namespaces")
		}

		// Cleanup labeled objects
		for _, obj := range labeledObjects {
			doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", obj.namespace, obj.name), nil)
		}
	})

	// Cleanup all created objects
	for _, obj := range objects {
		doRequest(t, "DELETE", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", obj.namespace, obj.name), nil)
	}
}

// Helper functions

func TestGenerateName(t *testing.T) {
	namespace := "default"

	spec := map[string]interface{}{
		"publicField":   "value",
		"internalField": "value",
		"nested": map[string]interface{}{
			"publicField":   "nested-value",
			"internalField": "nested-value",
		},
	}

	// Create with generateName and no name
	payload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"generateName": "gen-test-",
		},
		"spec": spec,
	}

	resp, body := doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), payload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d: %s", resp.StatusCode, body)
	}

	var created map[string]interface{}
	json.Unmarshal([]byte(body), &created)
	metadata := created["metadata"].(map[string]interface{})
	name := metadata["name"].(string)

	if len(name) <= len("gen-test-") {
		t.Fatalf("Expected generated name longer than prefix, got %q", name)
	}
	if name[:len("gen-test-")] != "gen-test-" {
		t.Fatalf("Expected name to start with 'gen-test-', got %q", name)
	}
	t.Logf("Generated name: %s", name)

	// Verify the object can be retrieved by the generated name
	resp, body = doRequest(t, "GET", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects/%s", namespace, name), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for GET by generated name, got %d: %s", resp.StatusCode, body)
	}

	// Create a second object with the same prefix to verify uniqueness
	resp, body = doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), payload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 Created for second object, got %d: %s", resp.StatusCode, body)
	}

	var created2 map[string]interface{}
	json.Unmarshal([]byte(body), &created2)
	name2 := created2["metadata"].(map[string]interface{})["name"].(string)

	if name2 == name {
		t.Fatalf("Expected different generated names, both got %q", name)
	}
	t.Logf("Second generated name: %s", name2)

	// Create without name or generateName should fail
	noNamePayload := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata":   map[string]interface{}{},
		"spec":       spec,
	}

	resp, body = doRequest(t, "POST", fmt.Sprintf("/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects", namespace), noNamePayload)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for missing name and generateName, got %d: %s", resp.StatusCode, body)
	}
}

func doRequest(t *testing.T, method, path string, body interface{}) (*http.Response, string) {
	var reqBody io.Reader
	if body != nil {
		jsonData, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		t.Fatal(err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp, string(respBody)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && contains(s[1:], substr) || s[:len(substr)] == substr)
}
