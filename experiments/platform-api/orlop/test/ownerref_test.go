package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TestOwnerReferenceValidation tests that invalid owner references are rejected.
func TestOwnerReferenceValidation(t *testing.T) {
	namespace := "default"

	// Create parent object
	parentBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "parent",
			"namespace": namespace,
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

	parentJSON, _ := json.Marshal(parentBody)
	createResp, err := http.Post(
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
		"application/json",
		bytes.NewBuffer(parentJSON),
	)
	if err != nil {
		t.Fatalf("Create parent request failed: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		t.Fatalf("Expected 201 Created for parent, got %d: %s", createResp.StatusCode, body)
	}

	var createdParent map[string]interface{}
	bodyBytes, _ := io.ReadAll(createResp.Body)
	json.Unmarshal(bodyBytes, &createdParent)
	parentUID := createdParent["metadata"].(map[string]interface{})["uid"].(string)

	// Try to create child with valid owner reference - should succeed
	validChildBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "valid-child",
			"namespace": namespace,
			"ownerReferences": []map[string]interface{}{
				{
					"apiVersion": "test.orlop.thetechnick.ninja/v1",
					"kind":       "Object",
					"name":       "parent",
					"uid":        parentUID,
				},
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

	validChildJSON, _ := json.Marshal(validChildBody)
	validResp, err := http.Post(
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
		"application/json",
		bytes.NewBuffer(validChildJSON),
	)
	if err != nil {
		t.Fatalf("Create valid child request failed: %v", err)
	}
	defer validResp.Body.Close()

	if validResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(validResp.Body)
		t.Fatalf("Expected 201 Created for valid child, got %d: %s", validResp.StatusCode, body)
	}

	var createdChild map[string]interface{}
	validBodyBytes, _ := io.ReadAll(validResp.Body)
	json.Unmarshal(validBodyBytes, &createdChild)
	ownerRefs := createdChild["metadata"].(map[string]interface{})["ownerReferences"].([]interface{})
	if len(ownerRefs) != 1 {
		t.Fatalf("expected 1 owner reference, got %d", len(ownerRefs))
	}

	// Try to create child with non-existent owner - should fail
	invalidChildBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "invalid-child",
			"namespace": namespace,
			"ownerReferences": []map[string]interface{}{
				{
					"apiVersion": "test.orlop.thetechnick.ninja/v1",
					"kind":       "Object",
					"name":       "nonexistent-owner",
					"uid":        "fake-uid",
				},
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

	invalidChildJSON, _ := json.Marshal(invalidChildBody)
	invalidResp, err := http.Post(
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
		"application/json",
		bytes.NewBuffer(invalidChildJSON),
	)
	if err != nil {
		t.Fatalf("Create invalid child request failed: %v", err)
	}
	defer invalidResp.Body.Close()

	if invalidResp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(invalidResp.Body)
		t.Fatalf("Expected 400 Bad Request for invalid owner, got %d: %s", invalidResp.StatusCode, body)
	}

	var status metav1.Status
	body, _ := io.ReadAll(invalidResp.Body)
	json.Unmarshal(body, &status)

	if status.Status != metav1.StatusFailure {
		t.Errorf("expected status 'Failure', got %s", status.Status)
	}
	t.Logf("Got expected error: %s", status.Message)
}

// TestCascadeDeletionForeground tests foreground cascade deletion.
func TestCascadeDeletionForeground(t *testing.T) {
	namespace := "default"

	// Create parent
	parentBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "parent-fg",
			"namespace": namespace,
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

	parentJSON, _ := json.Marshal(parentBody)
	createResp, err := http.Post(
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
		"application/json",
		bytes.NewBuffer(parentJSON),
	)
	if err != nil {
		t.Fatalf("Create parent request failed: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		t.Fatalf("Expected 201 Created for parent, got %d: %s", createResp.StatusCode, body)
	}

	var createdParent map[string]interface{}
	parentBodyBytes, _ := io.ReadAll(createResp.Body)
	json.Unmarshal(parentBodyBytes, &createdParent)
	parentUID := createdParent["metadata"].(map[string]interface{})["uid"].(string)

	// Create child with owner reference
	childBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "child-fg",
			"namespace": namespace,
			"ownerReferences": []map[string]interface{}{
				{
					"apiVersion": "test.orlop.thetechnick.ninja/v1",
					"kind":       "Object",
					"name":       "parent-fg",
					"uid":        parentUID,
				},
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

	childJSON, _ := json.Marshal(childBody)
	childResp, err := http.Post(
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
		"application/json",
		bytes.NewBuffer(childJSON),
	)
	if err != nil {
		t.Fatalf("Create child request failed: %v", err)
	}
	defer childResp.Body.Close()

	if childResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(childResp.Body)
		t.Fatalf("Expected 201 Created for child, got %d: %s", childResp.StatusCode, body)
	}

	// Delete parent with foreground propagation
	req, _ := http.NewRequest(
		"DELETE",
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/parent-fg?propagationPolicy=Foreground",
		nil,
	)
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Delete request failed: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(deleteResp.Body)
		t.Fatalf("Expected 200 OK for delete, got %d: %s", deleteResp.StatusCode, body)
	}

	// Parent should be deleted
	getParentResp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects/parent-fg")
	if err != nil {
		t.Fatalf("Get parent request failed: %v", err)
	}
	defer getParentResp.Body.Close()

	if getParentResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected parent to be deleted (404), got status %d", getParentResp.StatusCode)
	}

	// Child should be deleted synchronously with foreground deletion
	getChildResp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects/child-fg")
	if err != nil {
		t.Fatalf("Get child request failed: %v", err)
	}
	defer getChildResp.Body.Close()

	if getChildResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected child to be deleted (404) with foreground deletion, got status %d", getChildResp.StatusCode)
	}

	t.Log("Foreground cascade deletion: both parent and child deleted synchronously")
}

// TestCascadeDeletionOrphan tests orphan deletion policy.
func TestCascadeDeletionOrphan(t *testing.T) {
	namespace := "default"

	// Create parent
	parentBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "parent-orphan",
			"namespace": namespace,
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

	parentJSON, _ := json.Marshal(parentBody)
	createResp, err := http.Post(
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
		"application/json",
		bytes.NewBuffer(parentJSON),
	)
	if err != nil {
		t.Fatalf("Create parent request failed: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		t.Fatalf("Expected 201 Created for parent, got %d: %s", createResp.StatusCode, body)
	}

	var createdParent map[string]interface{}
	parentBodyBytes, _ := io.ReadAll(createResp.Body)
	json.Unmarshal(parentBodyBytes, &createdParent)
	parentUID := createdParent["metadata"].(map[string]interface{})["uid"].(string)

	// Create child with owner reference
	childBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "child-orphan",
			"namespace": namespace,
			"ownerReferences": []map[string]interface{}{
				{
					"apiVersion": "test.orlop.thetechnick.ninja/v1",
					"kind":       "Object",
					"name":       "parent-orphan",
					"uid":        parentUID,
				},
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

	childJSON, _ := json.Marshal(childBody)
	childResp, err := http.Post(
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
		"application/json",
		bytes.NewBuffer(childJSON),
	)
	if err != nil {
		t.Fatalf("Create child request failed: %v", err)
	}
	defer childResp.Body.Close()

	if childResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(childResp.Body)
		t.Fatalf("Expected 201 Created for child, got %d: %s", childResp.StatusCode, body)
	}

	// Delete parent with orphan propagation
	req, _ := http.NewRequest(
		"DELETE",
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/parent-orphan?propagationPolicy=Orphan",
		nil,
	)
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Delete request failed: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(deleteResp.Body)
		t.Fatalf("Expected 200 OK for delete, got %d: %s", deleteResp.StatusCode, body)
	}

	// Parent should be deleted
	getParentResp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects/parent-orphan")
	if err != nil {
		t.Fatalf("Get parent request failed: %v", err)
	}
	defer getParentResp.Body.Close()

	if getParentResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected parent to be deleted (404), got status %d", getParentResp.StatusCode)
	}

	// Child should still exist (orphaned)
	getChildResp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects/child-orphan")
	if err != nil {
		t.Fatalf("Get child request failed: %v", err)
	}
	defer getChildResp.Body.Close()

	if getChildResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getChildResp.Body)
		t.Fatalf("Expected child to still exist (200), got %d: %s", getChildResp.StatusCode, body)
	}

	var fetchedChild map[string]interface{}
	childBodyBytes, _ := io.ReadAll(getChildResp.Body)
	json.Unmarshal(childBodyBytes, &fetchedChild)

	// Child should have no owner references
	metadata := fetchedChild["metadata"].(map[string]interface{})
	ownerRefs, hasOwnerRefs := metadata["ownerReferences"]
	if hasOwnerRefs && ownerRefs != nil {
		refs := ownerRefs.([]interface{})
		if len(refs) != 0 {
			t.Errorf("expected child to be orphaned (no owner references), got %d owner references", len(refs))
		}
	}

	t.Log("Orphan deletion: parent deleted, child orphaned (owner reference removed)")
}

// TestUpdateOwnerReferenceValidation tests that updating an object with invalid owner references fails.
func TestUpdateOwnerReferenceValidation(t *testing.T) {
	namespace := "default"

	// Create object without owner references
	createBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "test-ownerref-update",
			"namespace": namespace,
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

	var created map[string]interface{}
	createdBodyBytes, _ := io.ReadAll(createResp.Body)
	json.Unmarshal(createdBodyBytes, &created)

	// Try to update with invalid owner reference
	metadata := created["metadata"].(map[string]interface{})
	metadata["ownerReferences"] = []map[string]interface{}{
		{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"name":       "nonexistent",
			"uid":        types.UID("fake-uid"),
		},
	}

	updateJSON, _ := json.Marshal(created)
	req, _ := http.NewRequest(
		"PUT",
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/test-ownerref-update",
		bytes.NewBuffer(updateJSON),
	)
	req.Header.Set("Content-Type", "application/json")
	updateResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Update request failed: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(updateResp.Body)
		t.Fatalf("Expected 400 Bad Request for invalid owner reference, got %d: %s", updateResp.StatusCode, body)
	}

	var status metav1.Status
	body, _ := io.ReadAll(updateResp.Body)
	json.Unmarshal(body, &status)

	t.Logf("Got expected error on update: %s", status.Message)
}

// TestCrossNamespaceOwnerReferenceRejected tests that owner references pointing
// to a different namespace are rejected on both Create and Update.
func TestCrossNamespaceOwnerReferenceRejected(t *testing.T) {
	namespace := "default"

	spec := map[string]interface{}{
		"publicField":   "value",
		"internalField": "value",
		"nested": map[string]interface{}{
			"publicField":   "nested-value",
			"internalField": "nested-value",
		},
	}

	// Create should reject cross-namespace owner reference
	body := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "cross-ns-child",
			"namespace": namespace,
			"ownerReferences": []map[string]interface{}{
				{
					"apiVersion": "test.orlop.thetechnick.ninja/v1",
					"kind":       "Object",
					"name":       "some-owner",
					"uid":        "fake-uid",
					"namespace":  "other-namespace",
				},
			},
		},
		"spec": spec,
	}

	bodyJSON, _ := json.Marshal(body)
	resp, err := http.Post(
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
		"application/json",
		bytes.NewBuffer(bodyJSON),
	)
	if err != nil {
		t.Fatalf("Create request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 400 Bad Request for cross-namespace owner reference, got %d: %s", resp.StatusCode, respBody)
	}

	var status metav1.Status
	respBody, _ := io.ReadAll(resp.Body)
	json.Unmarshal(respBody, &status)

	if status.Status != metav1.StatusFailure {
		t.Errorf("expected status 'Failure', got %s", status.Status)
	}
	if !strings.Contains(status.Message, "cross-namespace owner references are not allowed") {
		t.Errorf("expected cross-namespace error message, got: %s", status.Message)
	}
	t.Logf("Create correctly rejected: %s", status.Message)

	// Create an object without owner references first
	plainBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "cross-ns-update-target",
			"namespace": namespace,
		},
		"spec": spec,
	}

	plainJSON, _ := json.Marshal(plainBody)
	createResp, err := http.Post(
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
		"application/json",
		bytes.NewBuffer(plainJSON),
	)
	if err != nil {
		t.Fatalf("Create request failed: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(createResp.Body)
		t.Fatalf("Expected 201 Created, got %d: %s", createResp.StatusCode, respBody)
	}

	// Update should also reject cross-namespace owner reference
	updateBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      "cross-ns-update-target",
			"namespace": namespace,
			"ownerReferences": []map[string]interface{}{
				{
					"apiVersion": "test.orlop.thetechnick.ninja/v1",
					"kind":       "Object",
					"name":       "some-owner",
					"uid":        "fake-uid",
					"namespace":  "other-namespace",
				},
			},
		},
		"spec": spec,
	}

	updateJSON, _ := json.Marshal(updateBody)
	req, err := http.NewRequest(http.MethodPut,
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/cross-ns-update-target",
		bytes.NewBuffer(updateJSON),
	)
	if err != nil {
		t.Fatalf("Failed to create update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	updateResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Update request failed: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(updateResp.Body)
		t.Fatalf("Expected 400 Bad Request for cross-namespace owner reference on update, got %d: %s", updateResp.StatusCode, respBody)
	}

	var updateStatus metav1.Status
	updateRespBody, _ := io.ReadAll(updateResp.Body)
	json.Unmarshal(updateRespBody, &updateStatus)

	if !strings.Contains(updateStatus.Message, "cross-namespace owner references are not allowed") {
		t.Errorf("expected cross-namespace error message, got: %s", updateStatus.Message)
	}
	t.Logf("Update correctly rejected: %s", updateStatus.Message)
}
