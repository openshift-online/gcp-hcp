package maestroclient

import (
	"encoding/json"
	"testing"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/yaml"
)

// --- helpers ---

// mustJSON marshals v to JSON or panics.
func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

// bareNamespaceJSON returns a bare Namespace manifest as JSON.
func bareNamespaceJSON(t *testing.T, name string) []byte {
	t.Helper()
	return mustJSON(t, map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": name,
			"annotations": map[string]interface{}{
				constants.AnnotationGeneration: "1",
			},
		},
	})
}

// unmarshalManifestRaw unmarshals a workv1.Manifest.Raw back to a map.
func unmarshalManifestRaw(t *testing.T, m workv1.Manifest) map[string]interface{} {
	t.Helper()
	require.NotNil(t, m.Raw)
	var obj map[string]interface{}
	require.NoError(t, json.Unmarshal(m.Raw, &obj))
	return obj
}

// newTestManifestWork creates a ManifestWork with the given workload manifests.
func newTestManifestWork(name string, manifests []workv1.Manifest) *workv1.ManifestWork {
	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				constants.AnnotationGeneration: "1",
			},
			Labels: map[string]string{
				"test": "true",
			},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: manifests,
			},
		},
	}
}

// --- parseManifestWork tests ---

func TestParseManifestWork_JSON(t *testing.T) {
	mw := newTestManifestWork("test-mw", []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: bareNamespaceJSON(t, "my-ns")}},
	})
	data, err := json.Marshal(mw)
	require.NoError(t, err)

	parsed, err := parseManifestWork(data)
	require.NoError(t, err)
	assert.Equal(t, "test-mw", parsed.Name)
	assert.Equal(t, "1", parsed.Annotations[constants.AnnotationGeneration])
	assert.Equal(t, "true", parsed.Labels["test"])
	require.Len(t, parsed.Spec.Workload.Manifests, 1)
}

func TestParseManifestWork_YAML(t *testing.T) {
	mw := newTestManifestWork("yaml-mw", []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: bareNamespaceJSON(t, "yaml-ns")}},
	})
	jsonData, err := json.Marshal(mw)
	require.NoError(t, err)

	// Convert to YAML
	yamlData, err := yaml.JSONToYAML(jsonData)
	require.NoError(t, err)

	parsed, err := parseManifestWork(yamlData)
	require.NoError(t, err)
	assert.Equal(t, "yaml-mw", parsed.Name)
	require.Len(t, parsed.Spec.Workload.Manifests, 1)
}

func TestParseManifestWork_EmptyData(t *testing.T) {
	// Empty JSON produces a ManifestWork with no name - still parses successfully
	parsed, err := parseManifestWork([]byte("{}"))
	require.NoError(t, err)
	assert.Equal(t, "", parsed.Name)
}

func TestParseManifestWork_InvalidJSON(t *testing.T) {
	_, err := parseManifestWork([]byte("{invalid json"))
	require.Error(t, err)
}

func TestParseManifestWork_PreservesMetadata(t *testing.T) {
	mw := newTestManifestWork("my-manifestwork", []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: bareNamespaceJSON(t, "ns")}},
	})
	mw.Labels["extra"] = "label"
	mw.Annotations["extra"] = "annotation"

	data, err := json.Marshal(mw)
	require.NoError(t, err)

	parsed, err := parseManifestWork(data)
	require.NoError(t, err)
	assert.Equal(t, "my-manifestwork", parsed.Name)
	assert.Equal(t, "true", parsed.Labels["test"])
	assert.Equal(t, "label", parsed.Labels["extra"])
	assert.Equal(t, "1", parsed.Annotations[constants.AnnotationGeneration])
	assert.Equal(t, "annotation", parsed.Annotations["extra"])
}

func TestParseManifestWork_MultipleManifests(t *testing.T) {
	nsJSON := bareNamespaceJSON(t, "cluster-abc")
	cmJSON := mustJSON(t, map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "cluster-config",
			"namespace": "cluster-abc",
			"annotations": map[string]interface{}{
				constants.AnnotationGeneration: "1",
			},
		},
		"data": map[string]interface{}{"cluster_id": "abc"},
	})

	mw := newTestManifestWork("multi-mw", []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: nsJSON}},
		{RawExtension: runtime.RawExtension{Raw: cmJSON}},
	})

	data, err := json.Marshal(mw)
	require.NoError(t, err)

	parsed, err := parseManifestWork(data)
	require.NoError(t, err)
	require.Len(t, parsed.Spec.Workload.Manifests, 2)

	ns := unmarshalManifestRaw(t, parsed.Spec.Workload.Manifests[0])
	assert.Equal(t, "Namespace", ns["kind"])

	cm := unmarshalManifestRaw(t, parsed.Spec.Workload.Manifests[1])
	assert.Equal(t, "ConfigMap", cm["kind"])
}

// --- resolveTransportContext tests ---

func TestResolveTransportContext_Valid(t *testing.T) {
	c := &Client{}
	tc := &TransportContext{ConsumerName: "cluster-1"}
	result := c.resolveTransportContext(tc)
	require.NotNil(t, result)
	assert.Equal(t, "cluster-1", result.ConsumerName)
}

func TestResolveTransportContext_Nil(t *testing.T) {
	c := &Client{}
	result := c.resolveTransportContext(nil)
	assert.Nil(t, result)
}

func TestResolveTransportContext_WrongType(t *testing.T) {
	c := &Client{}
	result := c.resolveTransportContext("not-a-transport-context")
	assert.Nil(t, result)
}
