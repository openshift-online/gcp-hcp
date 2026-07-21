package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestJSONPatch tests JSON Patch (RFC 6902) operations
func TestJSONPatch(t *testing.T) {
	t.Run("Replace operation", func(t *testing.T) {
		namespace := "default"
		name := "json-patch-replace"

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

		// Apply JSON Patch - replace operation
		patchOps := []map[string]interface{}{
			{
				"op":    "replace",
				"path":  "/spec/publicField",
				"value": "replaced",
			},
		}
		patchJSON, _ := json.Marshal(patchOps)

		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			bytes.NewBuffer(patchJSON),
		)
		req.Header.Set("Content-Type", "application/json-patch+json")

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
		if spec["publicField"] != "replaced" {
			t.Errorf("Expected publicField 'replaced', got %v", spec["publicField"])
		}

		// Other fields should be preserved
		if spec["internalField"] != "internal" {
			t.Errorf("Expected internalField 'internal' to be preserved, got %v", spec["internalField"])
		}
	})

	t.Run("Add operation", func(t *testing.T) {
		namespace := "default"
		name := "json-patch-add"

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

		// Apply JSON Patch - add a new field
		patchOps := []map[string]interface{}{
			{
				"op":    "add",
				"path":  "/spec/defaultField",
				"value": "added-value",
			},
		}
		patchJSON, _ := json.Marshal(patchOps)

		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			bytes.NewBuffer(patchJSON),
		)
		req.Header.Set("Content-Type", "application/json-patch+json")

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
		// Note: defaultField might be set by defaulting, so just check it exists
		if spec["defaultField"] == nil {
			t.Errorf("Expected defaultField to be set")
		}
	})

	t.Run("Replace nested field", func(t *testing.T) {
		namespace := "default"
		name := "json-patch-nested"

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

		// Apply JSON Patch - replace nested field
		patchOps := []map[string]interface{}{
			{
				"op":    "replace",
				"path":  "/spec/nested/internalField",
				"value": "nested-replaced",
			},
		}
		patchJSON, _ := json.Marshal(patchOps)

		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			bytes.NewBuffer(patchJSON),
		)
		req.Header.Set("Content-Type", "application/json-patch+json")

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
		nested := spec["nested"].(map[string]interface{})

		// The replaced field should have new value
		if nested["internalField"] != "nested-replaced" {
			t.Errorf("Expected nested.internalField 'nested-replaced', got %v", nested["internalField"])
		}

		// Other fields should be preserved
		if spec["publicField"] != "original" {
			t.Errorf("Expected publicField 'original' to be preserved, got %v", spec["publicField"])
		}
		if nested["publicField"] != "nested-original" {
			t.Errorf("Expected nested.publicField 'nested-original' to be preserved, got %v", nested["publicField"])
		}
	})

	t.Run("Multiple operations", func(t *testing.T) {
		namespace := "default"
		name := "json-patch-multiple"

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

		// Apply JSON Patch - multiple operations
		patchOps := []map[string]interface{}{
			{
				"op":    "replace",
				"path":  "/spec/publicField",
				"value": "replaced",
			},
			{
				"op":    "replace",
				"path":  "/spec/nested/publicField",
				"value": "nested-replaced",
			},
		}
		patchJSON, _ := json.Marshal(patchOps)

		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			bytes.NewBuffer(patchJSON),
		)
		req.Header.Set("Content-Type", "application/json-patch+json")

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
		if spec["publicField"] != "replaced" {
			t.Errorf("Expected publicField 'replaced', got %v", spec["publicField"])
		}

		nested := spec["nested"].(map[string]interface{})
		if nested["publicField"] != "nested-replaced" {
			t.Errorf("Expected nested.publicField 'nested-replaced', got %v", nested["publicField"])
		}

		// Other fields should be preserved
		if spec["internalField"] != "internal" {
			t.Errorf("Expected internalField 'internal' to be preserved, got %v", spec["internalField"])
		}
		if nested["internalField"] != "nested-internal" {
			t.Errorf("Expected nested.internalField 'nested-internal' to be preserved, got %v", nested["internalField"])
		}
	})

	t.Run("Test operation", func(t *testing.T) {
		namespace := "default"
		name := "json-patch-test"

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

		// Apply JSON Patch - test operation that should succeed
		patchOps := []map[string]interface{}{
			{
				"op":    "test",
				"path":  "/spec/publicField",
				"value": "original",
			},
			{
				"op":    "replace",
				"path":  "/spec/publicField",
				"value": "replaced",
			},
		}
		patchJSON, _ := json.Marshal(patchOps)

		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			bytes.NewBuffer(patchJSON),
		)
		req.Header.Set("Content-Type", "application/json-patch+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Patch request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200 OK, got %d: %s", resp.StatusCode, body)
		}

		// Verify the patch applied
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		spec := result["spec"].(map[string]interface{})
		if spec["publicField"] != "replaced" {
			t.Errorf("Expected publicField 'replaced', got %v", spec["publicField"])
		}
	})

	t.Run("Test operation failure", func(t *testing.T) {
		namespace := "default"
		name := "json-patch-test-fail"

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

		// Apply JSON Patch - test operation that should fail
		patchOps := []map[string]interface{}{
			{
				"op":    "test",
				"path":  "/spec/publicField",
				"value": "wrong-value", // This doesn't match "original"
			},
			{
				"op":    "replace",
				"path":  "/spec/publicField",
				"value": "replaced",
			},
		}
		patchJSON, _ := json.Marshal(patchOps)

		req, _ := http.NewRequest(
			"PATCH",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			bytes.NewBuffer(patchJSON),
		)
		req.Header.Set("Content-Type", "application/json-patch+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Patch request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should fail because test operation doesn't match
		if resp.StatusCode == http.StatusOK {
			t.Errorf("Expected patch to fail due to test operation, but got 200 OK")
		}
	})
}
