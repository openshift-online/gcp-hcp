package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestStrategicMergePatch tests strategic merge patch operations
func TestStrategicMergePatch(t *testing.T) {
	t.Run("Basic strategic merge patch", func(t *testing.T) {
		namespace := "default"
		name := "strategic-patch-test"

		// Create initial object
		createBody := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"publicField":   "original",
				"internalField": "internal",
				"nested": map[string]interface{}{
					"publicField":   "nested-original",
					"internalField": "nested-internal",
				},
			},
		}
		createJSON, _ := json.Marshal(createBody)
		createResp, err := http.Post(
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
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

		// Apply strategic merge patch
		patchBody := map[string]interface{}{
			"spec": map[string]interface{}{
				"publicField": "patched",
			},
		}
		patchJSON, _ := json.Marshal(patchBody)

		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			bytes.NewBuffer(patchJSON),
		)
		req.Header.Set("Content-Type", "application/strategic-merge-patch+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Patch request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200 OK, got %d: %s", resp.StatusCode, body)
		}

		// Verify the patch
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spec := result["spec"].(map[string]interface{})
		if spec["publicField"] != "patched" {
			t.Errorf("Expected publicField 'patched', got %v", spec["publicField"])
		}

		// Verify other fields are preserved
		if spec["internalField"] != "internal" {
			t.Errorf("Expected internalField 'internal' to be preserved, got %v", spec["internalField"])
		}

		nested := spec["nested"].(map[string]interface{})
		if nested["publicField"] != "nested-original" {
			t.Errorf("Expected nested.publicField 'nested-original' to be preserved, got %v", nested["publicField"])
		}
	})

	t.Run("Strategic merge patch with nested updates", func(t *testing.T) {
		namespace := "default"
		name := "strategic-patch-nested"

		// Create initial object
		createBody := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"publicField":   "original",
				"internalField": "internal",
				"nested": map[string]interface{}{
					"publicField":   "nested-original",
					"internalField": "nested-internal",
				},
			},
		}
		createJSON, _ := json.Marshal(createBody)
		createResp, err := http.Post(
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
			"application/json",
			bytes.NewBuffer(createJSON),
		)
		if err != nil {
			t.Fatalf("Create request failed: %v", err)
		}
		defer createResp.Body.Close()

		// Patch nested field
		patchBody := map[string]interface{}{
			"spec": map[string]interface{}{
				"nested": map[string]interface{}{
					"publicField": "nested-patched",
				},
			},
		}
		patchJSON, _ := json.Marshal(patchBody)

		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			bytes.NewBuffer(patchJSON),
		)
		req.Header.Set("Content-Type", "application/strategic-merge-patch+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Patch request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200 OK, got %d: %s", resp.StatusCode, body)
		}

		// Verify the patch
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spec := result["spec"].(map[string]interface{})
		
		// Top-level fields should be preserved
		if spec["publicField"] != "original" {
			t.Errorf("Expected publicField 'original' to be preserved, got %v", spec["publicField"])
		}

		// Nested field should be updated
		nested := spec["nested"].(map[string]interface{})
		if nested["publicField"] != "nested-patched" {
			t.Errorf("Expected nested.publicField 'nested-patched', got %v", nested["publicField"])
		}

		// Other nested field should be preserved
		if nested["internalField"] != "nested-internal" {
			t.Errorf("Expected nested.internalField 'nested-internal' to be preserved, got %v", nested["internalField"])
		}
	})
}
