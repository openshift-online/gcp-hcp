package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestPatch tests PATCH endpoint with merge patch.
func TestPatch(t *testing.T) {
	// Create an object first
	createBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name": "patch-test",
		},
		"spec": map[string]interface{}{
			"publicField":   "original-value",
			"internalField": "original-internal",
			"nested": map[string]interface{}{
				"publicField":   "nested-original",
				"internalField": "nested-internal",
			},
		},
	}

	createJSON, _ := json.Marshal(createBody)
	resp, err := http.Post(
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects",
		"application/json",
		bytes.NewBuffer(createJSON),
	)
	if err != nil {
		t.Fatalf("Create request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 201, got %d: %s", resp.StatusCode, body)
	}

	// Test 1: PATCH with merge patch
	t.Run("MergePatch", func(t *testing.T) {
		patchBody := map[string]interface{}{
			"spec": map[string]interface{}{
				"publicField": "patched-value",
				"nested": map[string]interface{}{
					"publicField": "nested-patched",
				},
			},
		}

		patchJSON, _ := json.Marshal(patchBody)
		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/patch-test",
			bytes.NewBuffer(patchJSON),
		)
		req.Header.Set("Content-Type", "application/merge-patch+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PATCH request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spec := result["spec"].(map[string]interface{})
		if spec["publicField"] != "patched-value" {
			t.Errorf("Expected publicField 'patched-value', got %v", spec["publicField"])
		}

		// Internal field should be preserved
		if spec["internalField"] != "original-internal" {
			t.Errorf("Expected internalField 'original-internal', got %v", spec["internalField"])
		}

		nested := spec["nested"].(map[string]interface{})
		if nested["publicField"] != "nested-patched" {
			t.Errorf("Expected nested.publicField 'nested-patched', got %v", nested["publicField"])
		}

		// Nested internal field should be preserved
		if nested["internalField"] != "nested-internal" {
			t.Errorf("Expected nested.internalField 'nested-internal', got %v", nested["internalField"])
		}
	})

	// Test 2: PATCH without Content-Type (defaults to merge patch)
	t.Run("DefaultMergePatch", func(t *testing.T) {
		patchBody := map[string]interface{}{
			"spec": map[string]interface{}{
				"publicField": "default-patched",
			},
		}

		patchJSON, _ := json.Marshal(patchBody)
		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/patch-test",
			bytes.NewBuffer(patchJSON),
		)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PATCH request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spec := result["spec"].(map[string]interface{})
		if spec["publicField"] != "default-patched" {
			t.Errorf("Expected publicField 'default-patched', got %v", spec["publicField"])
		}
	})

	// Test 3: PATCH with strategic merge patch Content-Type
	t.Run("StrategicMergePatch", func(t *testing.T) {
		patchBody := map[string]interface{}{
			"spec": map[string]interface{}{
				"publicField": "strategic-patched",
			},
		}

		patchJSON, _ := json.Marshal(patchBody)
		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/patch-test",
			bytes.NewBuffer(patchJSON),
		)
		req.Header.Set("Content-Type", "application/strategic-merge-patch+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PATCH request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spec := result["spec"].(map[string]interface{})
		if spec["publicField"] != "strategic-patched" {
			t.Errorf("Expected publicField 'strategic-patched', got %v", spec["publicField"])
		}
	})

	// Test 4: PATCH non-existent object returns 404
	t.Run("NotFound", func(t *testing.T) {
		patchBody := map[string]interface{}{
			"spec": map[string]interface{}{
				"publicField": "value",
			},
		}

		patchJSON, _ := json.Marshal(patchBody)
		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/nonexistent",
			bytes.NewBuffer(patchJSON),
		)
		req.Header.Set("Content-Type", "application/merge-patch+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PATCH request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 404, got %d: %s", resp.StatusCode, body)
		}
	})
}
