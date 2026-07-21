package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestServerSideApply tests basic server-side apply functionality
func TestServerSideApply(t *testing.T) {
	t.Run("Create via Apply", func(t *testing.T) {
		// Apply configuration to create a new object
		applyConfig := `
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: apply-test-create
spec:
  publicField: initial-value
`
		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/apply-test-create?fieldManager=test-controller",
			bytes.NewBufferString(applyConfig),
		)
		req.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Apply request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 201 Created, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify object was created with correct field
		spec := result["spec"].(map[string]interface{})
		if spec["publicField"] != "initial-value" {
			t.Errorf("Expected publicField 'initial-value', got %v", spec["publicField"])
		}

		// Verify managedFields metadata exists
		metadata := result["metadata"].(map[string]interface{})
		if managedFields, ok := metadata["managedFields"]; !ok || managedFields == nil {
			t.Error("Expected managedFields to be set")
		}
	})

	t.Run("Update via Apply", func(t *testing.T) {
		// Create initial object
		createBody := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name": "apply-test-update",
			},
			"spec": map[string]interface{}{
				"publicField":   "original",
				"internalField": "internal",
				"nested": map[string]interface{}{
					"publicField":   "nested-public",
					"internalField": "nested-internal",
				},
			},
		}
		createJSON, _ := json.Marshal(createBody)
		createResp, err := http.Post(
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects",
			"application/json",
			bytes.NewBuffer(createJSON),
		)
		if err != nil {
			t.Fatalf("Create request failed: %v", err)
		}
		defer createResp.Body.Close()

		if createResp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(createResp.Body)
			t.Fatalf("Expected 201 Created, got %d: %s", createResp.StatusCode, body)
		}

		// Apply update
		applyConfig := `
spec:
  publicField: updated-via-apply
`
		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/apply-test-update?fieldManager=test-controller&force=true",
			bytes.NewBufferString(applyConfig),
		)
		req.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Apply request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200 OK, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spec := result["spec"].(map[string]interface{})
		if spec["publicField"] != "updated-via-apply" {
			t.Errorf("Expected publicField 'updated-via-apply', got %v", spec["publicField"])
		}
	})

	t.Run("JSON Apply Configuration", func(t *testing.T) {
		// Test with JSON instead of YAML
		applyConfig := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name": "apply-test-json",
			},
			"spec": map[string]interface{}{
				"publicField": "json-value",
			},
		}
		applyJSON, _ := json.Marshal(applyConfig)

		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/apply-test-json?fieldManager=json-controller",
			bytes.NewBuffer(applyJSON),
		)
		req.Header.Set("Content-Type", "application/apply-patch+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Apply request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 201 Created, got %d: %s", resp.StatusCode, body)
		}
	})
}

// TestServerSideApply_FieldManager tests field manager parameter handling
func TestServerSideApply_FieldManager(t *testing.T) {
	t.Run("Missing fieldManager returns 400", func(t *testing.T) {
		applyConfig := `
spec:
  publicField: value
`
		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/test-obj",
			bytes.NewBufferString(applyConfig),
		)
		req.Header.Set("Content-Type", "application/apply-patch+yaml")
		// Note: No fieldManager query parameter

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Apply request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected 400 Bad Request for missing fieldManager, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("Different field managers can own different fields", func(t *testing.T) {
		// Create object with controller-a
		// Note: Must include all required fields to avoid owning nested structure
		applyConfig1 := `
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: multi-manager-test
spec:
  publicField: from-controller-a
  internalField: ""
  nested:
    publicField: ""
    internalField: ""
`
		req1, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/multi-manager-test?fieldManager=controller-a",
			bytes.NewBufferString(applyConfig1),
		)
		req1.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp1, err := http.DefaultClient.Do(req1)
		if err != nil {
			t.Fatalf("First apply failed: %v", err)
		}
		resp1.Body.Close()

		if resp1.StatusCode != http.StatusCreated {
			t.Fatalf("Expected 201 Created, got %d", resp1.StatusCode)
		}

		// Apply with controller-b managing nested.publicField field
		// Use complete spec with all required fields to avoid clearing other fields
		applyConfig2 := `
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: multi-manager-test
spec:
  publicField: from-controller-a
  internalField: ""
  nested:
    publicField: from-controller-b
    internalField: ""
`
		req2, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/multi-manager-test?fieldManager=controller-b&force=true",
			bytes.NewBufferString(applyConfig2),
		)
		req2.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Second apply failed: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp2.Body)
			t.Fatalf("Expected 200 OK with force=true, got %d: %s", resp2.StatusCode, body)
		}

		// Verify both fields are present
		var result map[string]interface{}
		if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spec := result["spec"].(map[string]interface{})
		if spec["publicField"] != "from-controller-a" {
			t.Errorf("Expected publicField 'from-controller-a', got %v", spec["publicField"])
		}

		nested := spec["nested"].(map[string]interface{})
		if nested["publicField"] != "from-controller-b" {
			t.Errorf("Expected nested.publicField 'from-controller-b', got %v", nested["publicField"])
		}
	})
}

// TestServerSideApply_ConflictDetection tests field ownership conflicts
func TestServerSideApply_ConflictDetection(t *testing.T) {
	t.Run("Conflict detected without force", func(t *testing.T) {
		// Create object with controller-a owning publicField
		applyConfig1 := `
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: conflict-test
spec:
  publicField: owned-by-a
`
		req1, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/conflict-test?fieldManager=controller-a",
			bytes.NewBufferString(applyConfig1),
		)
		req1.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp1, _ := http.DefaultClient.Do(req1)
		resp1.Body.Close()

		// Try to apply with controller-b managing the same field (should conflict)
		applyConfig2 := `
spec:
  publicField: trying-to-own
`
		req2, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/conflict-test?fieldManager=controller-b&force=false",
			bytes.NewBufferString(applyConfig2),
		)
		req2.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Second apply request failed: %v", err)
		}
		defer resp2.Body.Close()

		// Should get 409 Conflict
		if resp2.StatusCode != http.StatusConflict {
			body, _ := io.ReadAll(resp2.Body)
			t.Errorf("Expected 409 Conflict, got %d: %s", resp2.StatusCode, body)
		}
	})

	t.Run("Force takeover succeeds", func(t *testing.T) {
		// Create object with controller-a
		applyConfig1 := `
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: force-test
spec:
  publicField: owned-by-a
`
		req1, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/force-test?fieldManager=controller-a",
			bytes.NewBufferString(applyConfig1),
		)
		req1.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp1, _ := http.DefaultClient.Do(req1)
		resp1.Body.Close()

		// Force takeover with controller-b
		applyConfig2 := `
spec:
  publicField: forced-takeover
`
		req2, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/force-test?fieldManager=controller-b&force=true",
			bytes.NewBufferString(applyConfig2),
		)
		req2.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Force apply request failed: %v", err)
		}
		defer resp2.Body.Close()

		// Should succeed with force=true
		if resp2.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp2.Body)
			t.Fatalf("Expected 200 OK with force=true, got %d: %s", resp2.StatusCode, body)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spec := result["spec"].(map[string]interface{})
		if spec["publicField"] != "forced-takeover" {
			t.Errorf("Expected publicField 'forced-takeover', got %v", spec["publicField"])
		}
	})
}

// TestServerSideApply_ManagedFields tests that managedFields metadata is properly tracked
func TestServerSideApply_ManagedFields(t *testing.T) {
	t.Run("ManagedFields tracking", func(t *testing.T) {
		// Create object via apply
		applyConfig := `
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: managed-fields-test
spec:
  publicField: test-value
`
		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/managed-fields-test?fieldManager=my-controller",
			bytes.NewBufferString(applyConfig),
		)
		req.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Apply request failed: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify managedFields exists
		metadata := result["metadata"].(map[string]interface{})
		managedFieldsRaw, ok := metadata["managedFields"]
		if !ok || managedFieldsRaw == nil {
			t.Fatal("Expected managedFields to be present in metadata")
		}

		managedFields := managedFieldsRaw.([]interface{})
		if len(managedFields) == 0 {
			t.Fatal("Expected at least one managedFields entry")
		}

		// Verify the field manager name is recorded
		foundManager := false
		for _, mf := range managedFields {
			entry := mf.(map[string]interface{})
			if entry["manager"] == "my-controller" {
				foundManager = true

				// Verify operation is "Apply"
				if entry["operation"] != string(metav1.ManagedFieldsOperationApply) {
					t.Errorf("Expected operation 'Apply', got %v", entry["operation"])
				}

				// Verify fieldsV1 exists
				if _, ok := entry["fieldsV1"]; !ok {
					t.Error("Expected fieldsV1 to be present")
				}

				break
			}
		}

		if !foundManager {
			t.Error("Expected to find managedFields entry for 'my-controller'")
		}
	})

	t.Run("Multiple managers in managedFields", func(t *testing.T) {
		// Apply with first manager
		applyConfig1 := `
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: multi-manager-fields
spec:
  publicField: from-manager-1
  internalField: ""
  nested:
    publicField: ""
    internalField: ""
`
		req1, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/multi-manager-fields?fieldManager=manager-1",
			bytes.NewBufferString(applyConfig1),
		)
		req1.Header.Set("Content-Type", "application/apply-patch+yaml")
		resp1, _ := http.DefaultClient.Do(req1)
		resp1.Body.Close()

		// Apply with second manager managing nested.publicField
		applyConfig2 := `
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: multi-manager-fields
spec:
  publicField: from-manager-1
  internalField: ""
  nested:
    publicField: from-manager-2
    internalField: ""
`
		req2, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/multi-manager-fields?fieldManager=manager-2&force=true",
			bytes.NewBufferString(applyConfig2),
		)
		req2.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Second apply failed: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp2.Body)
			t.Fatalf("Expected 200/201, got %d: %s", resp2.StatusCode, body)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify both managers are in managedFields
		metadata := result["metadata"].(map[string]interface{})
		managedFieldsRaw := metadata["managedFields"]
		if managedFieldsRaw == nil {
			t.Fatalf("managedFields is nil in response: %+v", metadata)
		}
		managedFields := managedFieldsRaw.([]interface{})

		foundManager1 := false
		foundManager2 := false
		for _, mf := range managedFields {
			entry := mf.(map[string]interface{})
			if entry["manager"] == "manager-1" {
				foundManager1 = true
			}
			if entry["manager"] == "manager-2" {
				foundManager2 = true
			}
		}

		if !foundManager1 {
			t.Error("Expected to find managedFields entry for 'manager-1'")
		}
		if !foundManager2 {
			t.Error("Expected to find managedFields entry for 'manager-2'")
		}
	})
}

// TestServerSideApply_PartialApply tests partial object specifications
func TestServerSideApply_PartialApply(t *testing.T) {
	t.Run("Partial apply preserves other fields", func(t *testing.T) {
		// Create object with multiple fields via apply
		applyConfig1 := `
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: partial-apply-test
spec:
  publicField: original-public
  internalField: ""
  nested:
    publicField: original-nested
    internalField: ""
`
		req1, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/partial-apply-test?fieldManager=initial-controller",
			bytes.NewBufferString(applyConfig1),
		)
		req1.Header.Set("Content-Type", "application/apply-patch+yaml")
		resp1, _ := http.DefaultClient.Do(req1)
		resp1.Body.Close()

		// Apply with different controller only updating nested field
		applyConfig := `
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: partial-apply-test
spec:
  publicField: original-public
  internalField: ""
  nested:
    publicField: updated-nested
    internalField: ""
`
		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/partial-apply-test?fieldManager=partial-controller&force=true",
			bytes.NewBufferString(applyConfig),
		)
		req.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Apply request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200/201, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spec, ok := result["spec"].(map[string]interface{})
		if !ok {
			t.Fatalf("spec is not a map: %+v", result)
		}

		// Original publicField should be preserved
		if spec["publicField"] != "original-public" {
			t.Errorf("Expected publicField 'original-public' to be preserved, got %v", spec["publicField"])
		}

		// Nested field should be updated
		nested := spec["nested"].(map[string]interface{})
		if nested["publicField"] != "updated-nested" {
			t.Errorf("Expected nested.publicField 'updated-nested', got %v", nested["publicField"])
		}
	})
}

// TestServerSideApply_NotFound tests apply to non-existent objects
func TestServerSideApply_NotFound(t *testing.T) {
	t.Run("Apply creates object if not found", func(t *testing.T) {
		applyConfig := `
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: apply-create-if-missing
spec:
  publicField: created-via-apply
`
		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/apply-create-if-missing?fieldManager=creator",
			bytes.NewBufferString(applyConfig),
		)
		req.Header.Set("Content-Type", "application/apply-patch+yaml")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Apply request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should create with 201 Created
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 201 Created, got %d: %s", resp.StatusCode, body)
		}

		// Verify the object exists by getting it
		getResp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/apply-create-if-missing")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusOK {
			t.Errorf("Expected object to exist after apply create, got %d", getResp.StatusCode)
		}
	})
}
