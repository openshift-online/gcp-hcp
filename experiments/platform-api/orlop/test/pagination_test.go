package test

import (
	"encoding/json"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"
)

// TestPagination tests basic pagination with Limit and Continue parameters.
func TestPagination(t *testing.T) {
	// Use a unique namespace to avoid interference from other tests
	namespace := "pagination-test"

	// Create 10 test objects for pagination
	for i := 0; i < 10; i++ {
		obj := map[string]interface{}{
			"apiVersion": "test.orlop.thetechnick.ninja/v1",
			"kind":       "Object",
			"metadata": map[string]interface{}{
				"name":      fmt.Sprintf("page-test-%02d", i),
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"publicField":   fmt.Sprintf("value-%d", i),
				"internalField": "internal",
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
			t.Fatalf("Failed to create object: %v", err)
		}
		resp.Body.Close()
	}

	t.Run("List with limit", func(t *testing.T) {
		// List with limit=3
		resp, err := http.Get(baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects?limit=3")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer resp.Body.Close()

		var list map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &list)

		items := list["items"].([]interface{})
		if len(items) != 3 {
			t.Errorf("Expected 3 items with limit=3, got %d", len(items))
		}

		// Check if continue token is present
		metadata := list["metadata"].(map[string]interface{})
		continueToken, hasContinue := metadata["continue"]
		if !hasContinue {
			t.Error("Expected continue token in metadata")
		}
		t.Logf("Got continue token: %v", continueToken)

		// Check remainingItemCount
		if remaining, ok := metadata["remainingItemCount"]; ok {
			t.Logf("Remaining items: %v", remaining)
		}
	})

	t.Run("Paginate through all items", func(t *testing.T) {
		allItems := []string{}
		continueToken := ""
		pageCount := 0

		for {
			pageCount++
			url := baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/" + namespace + "/objects?limit=3"
			if continueToken != "" {
				url += "&continue=" + continueToken
			}

			resp, err := http.Get(url)
			if err != nil {
				t.Fatalf("GET request failed: %v", err)
			}

			var list map[string]interface{}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			json.Unmarshal(bodyBytes, &list)

			items := list["items"].([]interface{})
			for _, item := range items {
				itemMap := item.(map[string]interface{})
				metadata := itemMap["metadata"].(map[string]interface{})
				allItems = append(allItems, metadata["name"].(string))
			}

			// Check for next page
			listMetadata := list["metadata"].(map[string]interface{})
			if nextContinue, ok := listMetadata["continue"]; ok && nextContinue != nil {
				continueToken = nextContinue.(string)
			} else {
				break
			}

			if pageCount > 10 {
				t.Fatal("Too many pages, possible infinite loop")
			}
		}

		t.Logf("Paginated through %d pages, got %d items", pageCount, len(allItems))

		// Should have at least 10 items (the ones we created)
		if len(allItems) < 10 {
			t.Errorf("Expected at least 10 items, got %d", len(allItems))
		}
	})

	// Cleanup
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("page-test-%02d", i)
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
