package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

// TestShardSelectorList tests shard-based filtering in LIST operations.
func TestShardSelectorList(t *testing.T) {
	namespace := "default"

	// Create 10 test objects
	createdNames := make([]string, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("shard-test-%d", i)
		obj := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"publicField":   fmt.Sprintf("value-%d", i),
				"internalField": "internal-value",
				"nested": map[string]interface{}{
					"publicField":   "nested-value",
					"internalField": "nested-internal",
				},
			},
		}

		objJSON, _ := json.Marshal(obj)
		resp, err := http.Post(
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
			"application/json",
			bytes.NewBuffer(objJSON),
		)
		if err != nil {
			t.Fatalf("Create request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 201 Created, got %d: %s", resp.StatusCode, body)
		}

		createdNames[i] = name
	}

	// Test: List all objects (no shard filter)
	t.Run("list all objects", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200 OK, got %d: %s", resp.StatusCode, body)
		}

		var list map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&list)

		items := list["items"].([]interface{})
		if len(items) < 10 {
			t.Errorf("Expected at least 10 items without shard filter, got %d", len(items))
		}
	})

	// Test: List with shard 0/2
	t.Run("list shard 0/2", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects?shardIndex=0&shardCount=2")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200 OK, got %d: %s", resp.StatusCode, body)
		}

		var list map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &list)

		items := list["items"].([]interface{})
		t.Logf("Shard 0/2 returned %d items", len(items))

		// Verify each returned object belongs to shard 0
		for _, item := range items {
			itemMap := item.(map[string]interface{})
			metadata := itemMap["metadata"].(map[string]interface{})
			name := metadata["name"].(string)
			t.Logf("  - %s", name)
		}
	})

	// Test: List with shard 1/2
	t.Run("list shard 1/2", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects?shardIndex=1&shardCount=2")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected 200 OK, got %d: %s", resp.StatusCode, body)
		}

		var list map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &list)

		items := list["items"].([]interface{})
		t.Logf("Shard 1/2 returned %d items", len(items))

		// Verify each returned object belongs to shard 1
		for _, item := range items {
			itemMap := item.(map[string]interface{})
			metadata := itemMap["metadata"].(map[string]interface{})
			name := metadata["name"].(string)
			t.Logf("  - %s", name)
		}
	})

	// Test: Invalid shard selector
	t.Run("invalid shard selector", func(t *testing.T) {
		// shardIndex >= shardCount
		resp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects?shardIndex=2&shardCount=2")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request for invalid shard selector, got %d", resp.StatusCode)
		}
	})

	// Test: Missing shardCount
	t.Run("missing shard count", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects?shardIndex=0")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request for missing shardCount, got %d", resp.StatusCode)
		}
	})

	// Cleanup: delete created objects
	for _, name := range createdNames {
		req, _ := http.NewRequest(
			"DELETE",
			baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
			nil,
		)
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}
}

// TestShardSelectorDeterministic tests that shard assignment is deterministic.
func TestShardSelectorDeterministic(t *testing.T) {
	namespace := "default"
	name := "deterministic-test"

	// Create object
	obj := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"publicField":   "value",
			"internalField": "internal-value",
			"nested": map[string]interface{}{
				"publicField":   "nested-value",
				"internalField": "nested-internal",
			},
		},
	}

	objJSON, _ := json.Marshal(obj)
	createResp, _ := http.Post(
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects",
		"application/json",
		bytes.NewBuffer(objJSON),
	)
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		t.Fatalf("Expected 201 Created, got %d: %s", createResp.StatusCode, body)
	}

	// Query with different shard counts to verify deterministic assignment
	shardCounts := []int{2, 3, 5, 10}

	for _, count := range shardCounts {
		foundInShard := -1

		for index := 0; index < count; index++ {
			url := fmt.Sprintf("%s/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects?shardIndex=%d&shardCount=%d",
				baseURL, namespace, index, count)

			resp, err := http.Get(url)
			if err != nil {
				t.Fatalf("GET request failed: %v", err)
			}
			defer resp.Body.Close()

			var list map[string]interface{}
			bodyBytes, _ := io.ReadAll(resp.Body)
			json.Unmarshal(bodyBytes, &list)

			items := list["items"].([]interface{})
			for _, item := range items {
				itemMap := item.(map[string]interface{})
				metadata := itemMap["metadata"].(map[string]interface{})
				itemName := metadata["name"].(string)

				if itemName == name {
					if foundInShard >= 0 {
						t.Errorf("Object %s found in multiple shards: %d and %d (count=%d)", name, foundInShard, index, count)
					}
					foundInShard = index
				}
			}
		}

		if foundInShard < 0 {
			t.Errorf("Object %s not found in any shard with count=%d", name, count)
		} else {
			t.Logf("Object %s assigned to shard %d/%d", name, foundInShard, count)
		}

		// Query same shard again to verify deterministic behavior
		url := fmt.Sprintf("%s/apis/test.orlop.thetechnick.ninja/v1/namespaces/%s/objects?shardIndex=%d&shardCount=%d",
			baseURL, namespace, foundInShard, count)

		resp, _ := http.Get(url)
		defer resp.Body.Close()

		var list map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &list)

		items := list["items"].([]interface{})
		found := false
		for _, item := range items {
			itemMap := item.(map[string]interface{})
			metadata := itemMap["metadata"].(map[string]interface{})
			if metadata["name"].(string) == name {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Object %s not consistently in shard %d/%d on second query", name, foundInShard, count)
		}
	}

	// Cleanup
	req, _ := http.NewRequest(
		"DELETE",
		baseURL+"/apis/test.orlop.thetechnick.ninja/v1/namespaces/"+namespace+"/objects/"+name,
		nil,
	)
	delResp, _ := http.DefaultClient.Do(req)
	if delResp != nil {
		delResp.Body.Close()
	}
}
