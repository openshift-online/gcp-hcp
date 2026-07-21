package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestKubectlEdit tests that kubectl edit works with our PATCH endpoint.
func TestKubectlEdit(t *testing.T) {
	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Skip("kubectl not found in PATH, skipping")
	}

	// Create a temporary directory for kubeconfig and editor script
	tmpDir := t.TempDir()

	// Create kubeconfig
	kubeconfig := filepath.Join(tmpDir, "kubeconfig.yaml")
	kubeconfigContent := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
  name: orlop
contexts:
- context:
    cluster: orlop
    user: admin
  name: orlop
current-context: orlop
users:
- name: admin
`, baseURL)

	if err := os.WriteFile(kubeconfig, []byte(kubeconfigContent), 0644); err != nil {
		t.Fatalf("Failed to write kubeconfig: %v", err)
	}

	// Create an object first
	createBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name": "kubectl-edit-test",
		},
		"spec": map[string]interface{}{
			"publicField":   "original-value",
			"internalField": "internal-value",
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

	// Create a custom editor script that modifies the object
	editorScript := filepath.Join(tmpDir, "editor.sh")
	editorContent := `#!/bin/bash
# This script simulates editing the object
FILE="$1"

# Show what we're editing for debugging
cat "$FILE" > /tmp/kubectl-edit-before.yaml

# Use sed to replace the value - be more specific to avoid breaking YAML
sed -i 's/publicField: original-value/publicField: kubectl-edited-value/g' "$FILE"
sed -i 's/publicField: nested-original/publicField: kubectl-nested-edited/g' "$FILE"

# Show result
cat "$FILE" > /tmp/kubectl-edit-after.yaml
`

	if err := os.WriteFile(editorScript, []byte(editorContent), 0755); err != nil {
		t.Fatalf("Failed to write editor script: %v", err)
	}

	// Run kubectl edit with custom editor and full API path
	// Client-side validation enabled - server returns 406 for protobuf requests
	cmd := exec.Command("kubectl",
		"--kubeconfig", kubeconfig,
		"edit",
		"objects.test.orlop.thetechnick.ninja/kubectl-edit-test",
		"-n", "default",
	)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("KUBECONFIG=%s", kubeconfig),
		fmt.Sprintf("KUBE_EDITOR=%s", editorScript),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Logf("kubectl stdout: %s", stdout.String())
		t.Logf("kubectl stderr: %s", stderr.String())
		t.Fatalf("kubectl edit failed: %v", err)
	}

	// Verify the edit was successful
	output := stdout.String()
	if !strings.Contains(output, "edited") {
		t.Logf("kubectl output: %s", output)
		// kubectl might not output "edited" in some versions, so just warn
		t.Logf("Warning: kubectl output doesn't contain 'edited'")
	}

	// Get the object to verify changes
	getResp, err := http.Get(
		baseURL + "/apis/test.orlop.thetechnick.ninja/v1/namespaces/default/objects/kubectl-edit-test",
	)
	if err != nil {
		t.Fatalf("Get request failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		t.Fatalf("Expected 200, got %d: %s", getResp.StatusCode, body)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(getResp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	spec := result["spec"].(map[string]interface{})

	// Verify publicField was edited by kubectl
	if spec["publicField"] != "kubectl-edited-value" {
		t.Errorf("Expected publicField 'kubectl-edited-value', got %v", spec["publicField"])
	}

	// Verify internal field was preserved (not shown in kubectl edit for public API)
	if spec["internalField"] != "internal-value" {
		t.Errorf("Expected internalField 'internal-value', got %v", spec["internalField"])
	}

	// Verify nested field was edited
	nested := spec["nested"].(map[string]interface{})
	if nested["publicField"] != "kubectl-nested-edited" {
		t.Errorf("Expected nested.publicField 'kubectl-nested-edited', got %v", nested["publicField"])
	}

	// Verify nested internal field was preserved
	if nested["internalField"] != "nested-internal" {
		t.Errorf("Expected nested.internalField 'nested-internal', got %v", nested["internalField"])
	}

	// Verify resource version was incremented (edit should trigger a change)
	metadata := result["metadata"].(map[string]interface{})
	resourceVersion := metadata["resourceVersion"].(string)
	if resourceVersion == "1" {
		t.Errorf("Expected resourceVersion > 1 after edit, got %v", resourceVersion)
	}
}

// TestKubectlGet tests that kubectl get works.
func TestKubectlGet(t *testing.T) {
	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Skip("kubectl not found in PATH, skipping")
	}

	// Create a temporary directory for kubeconfig
	tmpDir := t.TempDir()

	// Create kubeconfig
	kubeconfig := filepath.Join(tmpDir, "kubeconfig.yaml")
	kubeconfigContent := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
  name: orlop
contexts:
- context:
    cluster: orlop
    user: admin
  name: orlop
current-context: orlop
users:
- name: admin
`, baseURL)

	if err := os.WriteFile(kubeconfig, []byte(kubeconfigContent), 0644); err != nil {
		t.Fatalf("Failed to write kubeconfig: %v", err)
	}

	// Create an object
	createBody := map[string]interface{}{
		"apiVersion": "test.orlop.thetechnick.ninja/v1",
		"kind":       "Object",
		"metadata": map[string]interface{}{
			"name": "kubectl-get-test",
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

	// First, check if kubectl can discover APIs
	apiCmd := exec.Command("kubectl",
		"--kubeconfig", kubeconfig,
		"api-resources",
		"-v=8",
	)
	apiOut, apiErr := apiCmd.CombinedOutput()
	t.Logf("kubectl api-resources output:\n%s", string(apiOut))
	if apiErr != nil {
		t.Logf("kubectl api-resources error: %v", apiErr)
	}

	// Test kubectl get with full API path
	cmd := exec.Command("kubectl",
		"--kubeconfig", kubeconfig,
		"get", "objects.test.orlop.thetechnick.ninja/kubectl-get-test",
		"-n", "default",
		"-o", "json",
		"-v=8",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Logf("kubectl stdout: %s", stdout.String())
		t.Logf("kubectl stderr: %s", stderr.String())
		t.Fatalf("kubectl get failed: %v", err)
	}

	// Parse output
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse kubectl output: %v", err)
	}

	// Verify object
	metadata := result["metadata"].(map[string]interface{})
	if metadata["name"] != "kubectl-get-test" {
		t.Errorf("Expected name 'kubectl-get-test', got %v", metadata["name"])
	}

	spec := result["spec"].(map[string]interface{})
	if spec["publicField"] != "test-value" {
		t.Errorf("Expected publicField 'test-value', got %v", spec["publicField"])
	}
}
