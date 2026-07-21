package apply_test

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This test demonstrates how managedFields work in Kubernetes objects.
// It shows the data structure that server-side apply needs to maintain.

func TestManagedFields_Structure(t *testing.T) {
	// Example object with managedFields after two controllers applied changes
	objectJSON := `{
  "apiVersion": "test.orlop.thetechnick.ninja/v1",
  "kind": "Object",
  "metadata": {
    "name": "example",
    "namespace": "default",
    "managedFields": [
      {
        "manager": "controller-a",
        "operation": "Apply",
        "apiVersion": "test.orlop.thetechnick.ninja/v1",
        "time": "2026-07-09T10:00:00Z",
        "fieldsType": "FieldsV1",
        "fieldsV1": {
          "f:spec": {
            "f:replicas": {}
          }
        }
      },
      {
        "manager": "controller-b",
        "operation": "Apply",
        "apiVersion": "test.orlop.thetechnick.ninja/v1",
        "time": "2026-07-09T10:05:00Z",
        "fieldsType": "FieldsV1",
        "fieldsV1": {
          "f:spec": {
            "f:image": {},
            "f:nested": {
              "f:publicField": {}
            }
          }
        }
      },
      {
        "manager": "kubectl",
        "operation": "Update",
        "apiVersion": "test.orlop.thetechnick.ninja/v1",
        "time": "2026-07-09T10:10:00Z",
        "fieldsType": "FieldsV1",
        "fieldsV1": {
          "f:metadata": {
            "f:labels": {
              "f:app": {}
            }
          }
        }
      }
    ]
  },
  "spec": {
    "replicas": 3,
    "image": "nginx:1.20",
    "nested": {
      "publicField": "value"
    }
  }
}`

	var obj struct {
		Metadata metav1.ObjectMeta `json:"metadata"`
		Spec     map[string]any    `json:"spec"`
	}

	if err := json.Unmarshal([]byte(objectJSON), &obj); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify managed fields are present
	if len(obj.Metadata.ManagedFields) != 3 {
		t.Errorf("Expected 3 managed field entries, got %d", len(obj.Metadata.ManagedFields))
	}

	// Check controller-a owns spec.replicas
	controllerA := obj.Metadata.ManagedFields[0]
	if controllerA.Manager != "controller-a" {
		t.Errorf("Expected manager 'controller-a', got %s", controllerA.Manager)
	}
	if controllerA.Operation != "Apply" {
		t.Errorf("Expected operation 'Apply', got %s", controllerA.Operation)
	}

	// Check controller-b owns spec.image and spec.nested.publicField
	controllerB := obj.Metadata.ManagedFields[1]
	if controllerB.Manager != "controller-b" {
		t.Errorf("Expected manager 'controller-b', got %s", controllerB.Manager)
	}

	// Check kubectl used Update operation (not Apply)
	kubectl := obj.Metadata.ManagedFields[2]
	if kubectl.Manager != "kubectl" {
		t.Errorf("Expected manager 'kubectl', got %s", kubectl.Manager)
	}
	if kubectl.Operation != "Update" {
		t.Errorf("Expected operation 'Update', got %s", kubectl.Operation)
	}

	t.Logf("Successfully parsed object with managedFields:")
	t.Logf("  - controller-a manages: spec.replicas")
	t.Logf("  - controller-b manages: spec.image, spec.nested.publicField")
	t.Logf("  - kubectl manages: metadata.labels.app")
}

// Example of how conflict detection works
func TestManagedFields_ConflictScenario(t *testing.T) {
	t.Log("Scenario: Two controllers try to manage the same field")
	t.Log("")
	t.Log("1. controller-a applies: spec.replicas = 3")
	t.Log("   Result: controller-a now owns spec.replicas")
	t.Log("")
	t.Log("2. controller-b applies: spec.replicas = 5 (without force)")
	t.Log("   Result: 409 Conflict - spec.replicas is owned by controller-a")
	t.Log("")
	t.Log("3. controller-b applies: spec.replicas = 5 (with force=true)")
	t.Log("   Result: 200 OK - controller-b takes ownership of spec.replicas")
	t.Log("")
	t.Log("After step 3, managedFields shows:")
	t.Log("  - controller-b owns spec.replicas (controller-a no longer owns it)")
}

// Example of managedFields for nested structures
func TestManagedFields_NestedFields(t *testing.T) {
	fieldsV1JSON := `{
  "f:spec": {
    "f:nested": {
      "f:publicField": {},
      "f:array": {
        "k:{\"name\":\"item1\"}": {
          "f:name": {},
          "f:value": {}
        }
      }
    }
  }
}`

	var fieldsV1 map[string]any
	if err := json.Unmarshal([]byte(fieldsV1JSON), &fieldsV1); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	t.Log("FieldsV1 structure for nested fields:")
	t.Log("  - 'f:' prefix indicates a field")
	t.Log("  - 'k:' prefix indicates an array item by key")
	t.Log("  - Empty {} means the field is owned")
	t.Log("  - Nested structure mirrors the object structure")
}
