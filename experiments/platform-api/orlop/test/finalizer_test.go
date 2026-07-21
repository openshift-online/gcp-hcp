package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

// TestFinalizerDeletion tests the finalizer deletion flow
func TestFinalizerDeletion(t *testing.T) {
	t.Run("Delete without finalizers - immediate deletion", func(t *testing.T) {
		namespace := "default"
		name := "finalizer-test-no-finalizers"

		// Create object without finalizers
		createBody := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"publicField":   "value",
				"internalField": "value",
				"nested": map[string]interface{}{
					"publicField":   "nested-value",
					"internalField": "nested-value",
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

		// Delete object
		req, _ := http.NewRequest(
			"DELETE",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			nil,
		)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Delete request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200 OK, got %d: %s", resp.StatusCode, body)
		}

		// Verify object is deleted
		getResp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects/" + name)
		if err != nil {
			t.Fatalf("Get request failed: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 Not Found after deletion, got %d", getResp.StatusCode)
		}
	})

	t.Run("Delete with finalizers - soft deletion", func(t *testing.T) {
		namespace := "default"
		name := "finalizer-test-with-finalizers"

		// Create object with finalizers
		createBody := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name": name,
				"finalizers": []string{
					"test.orlop.thetechnick.ninja/finalizer-1",
					"test.orlop.thetechnick.ninja/finalizer-2",
				},
			},
			"spec": map[string]interface{}{
				"publicField":   "value",
				"internalField": "value",
				"nested": map[string]interface{}{
					"publicField":   "nested-value",
					"internalField": "nested-value",
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

		// Delete object (should set deletionTimestamp)
		req, _ := http.NewRequest(
			"DELETE",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			nil,
		)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Delete request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200 OK, got %d: %s", resp.StatusCode, body)
		}

		// Verify response contains the object with deletionTimestamp
		var deletedObj map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&deletedObj); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		metadata := deletedObj["metadata"].(map[string]interface{})
		if metadata["deletionTimestamp"] == nil {
			t.Errorf("Expected deletionTimestamp to be set")
		}

		finalizers := metadata["finalizers"].([]interface{})
		if len(finalizers) != 2 {
			t.Errorf("Expected 2 finalizers, got %d", len(finalizers))
		}

		// Verify object still exists (soft deleted)
		getResp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects/" + name)
		if err != nil {
			t.Fatalf("Get request failed: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK (object should still exist), got %d", getResp.StatusCode)
		}

		var getObj map[string]interface{}
		if err := json.NewDecoder(getResp.Body).Decode(&getObj); err != nil {
			t.Fatalf("Failed to decode get response: %v", err)
		}

		getMetadata := getObj["metadata"].(map[string]interface{})
		if getMetadata["deletionTimestamp"] == nil {
			t.Errorf("Expected deletionTimestamp to be present on soft-deleted object")
		}
	})

	t.Run("Remove finalizers - triggers hard deletion", func(t *testing.T) {
		namespace := "default"
		name := "finalizer-test-removal"

		// Create object with one finalizer
		createBody := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name": name,
				"finalizers": []string{
					"test.orlop.thetechnick.ninja/my-finalizer",
				},
			},
			"spec": map[string]interface{}{
				"publicField":   "value",
				"internalField": "value",
				"nested": map[string]interface{}{
					"publicField":   "nested-value",
					"internalField": "nested-value",
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

		var created map[string]interface{}
		json.NewDecoder(createResp.Body).Decode(&created)
		_ = created["metadata"].(map[string]interface{})["resourceVersion"].(string)

		// Soft delete (set deletionTimestamp)
		req, _ := http.NewRequest(
			"DELETE",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			nil,
		)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Delete request failed: %v", err)
		}
		resp.Body.Close()

		// Get the object to verify soft deletion
		getResp, _ := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects/" + name)
		var softDeleted map[string]interface{}
		json.NewDecoder(getResp.Body).Decode(&softDeleted)
		getResp.Body.Close()

		metadata := softDeleted["metadata"].(map[string]interface{})
		if metadata["deletionTimestamp"] == nil {
			t.Fatalf("Expected deletionTimestamp to be set after soft delete")
		}
		updatedResourceVersion := metadata["resourceVersion"].(string)

		// Remove the finalizer (should trigger hard deletion)
		updateBody := softDeleted
		updateMetadata := updateBody["metadata"].(map[string]interface{})
		updateMetadata["finalizers"] = []string{} // Remove all finalizers
		updateMetadata["resourceVersion"] = updatedResourceVersion

		updateJSON, _ := json.Marshal(updateBody)
		updateReq, _ := http.NewRequest(
			"PUT",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			bytes.NewBuffer(updateJSON),
		)
		updateReq.Header.Set("Content-Type", "application/json")

		updateResp, err := http.DefaultClient.Do(updateReq)
		if err != nil {
			t.Fatalf("Update request failed: %v", err)
		}
		defer updateResp.Body.Close()

		if updateResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(updateResp.Body)
			t.Fatalf("Expected 200 OK for finalizer removal, got %d: %s", updateResp.StatusCode, body)
		}

		// The response should be a Status object indicating deletion
		var result map[string]interface{}
		if err := json.NewDecoder(updateResp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result["kind"] != "Status" {
			t.Errorf("Expected Status response after finalizer removal, got %v", result["kind"])
		}

		// Wait a moment for deletion to complete
		time.Sleep(100 * time.Millisecond)

		// Verify object is hard deleted
		finalGetResp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects/" + name)
		if err != nil {
			t.Fatalf("Final get request failed: %v", err)
		}
		defer finalGetResp.Body.Close()

		if finalGetResp.StatusCode != http.StatusNotFound {
			body, _ := io.ReadAll(finalGetResp.Body)
			t.Errorf("Expected 404 Not Found after finalizer removal, got %d: %s", finalGetResp.StatusCode, body)
		}
	})

	t.Run("Second delete on soft-deleted object - idempotent", func(t *testing.T) {
		namespace := "default"
		name := "finalizer-test-idempotent"

		// Create object with finalizer
		createBody := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name": name,
				"finalizers": []string{
					"test.orlop.thetechnick.ninja/finalizer",
				},
			},
			"spec": map[string]interface{}{
				"publicField":   "value",
				"internalField": "value",
				"nested": map[string]interface{}{
					"publicField":   "nested-value",
					"internalField": "nested-value",
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
		createResp.Body.Close()

		// First delete
		req1, _ := http.NewRequest(
			"DELETE",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			nil,
		)
		resp1, err := http.DefaultClient.Do(req1)
		if err != nil {
			t.Fatalf("First delete request failed: %v", err)
		}
		resp1.Body.Close()

		// Second delete (should still succeed but not change anything)
		req2, _ := http.NewRequest(
			"DELETE",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			nil,
		)
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("Second delete request failed: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp2.Body)
			t.Fatalf("Expected 200 OK for second delete, got %d: %s", resp2.StatusCode, body)
		}

		// Object should still exist with deletionTimestamp
		getResp, _ := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects/" + name)
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusOK {
			t.Errorf("Expected object to still exist after second delete, got %d", getResp.StatusCode)
		}
	})
}
