package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	workv1 "open-cluster-management.io/api/work/v1"
)

func TestBuild_HappyPath(t *testing.T) {
	input := Input{
		NodePoolID:         "np-001",
		NodePoolName:       "my-nodepool",
		NodePoolGeneration: 3,
		ClusterID:          "cluster-abc",
		ClusterName:        "my-cluster",
		Replicas:           2,
		MachineType:        "n2-standard-8",
		GCPRegion:          "us-central1",
		Zone:               "us-central1-b",
		GCPSubnet:          "my-subnet",
		DiskSizeGB:         200,
		DiskType:           "pd-standard",
		ReleaseImage:       "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
	}

	mw, err := Build(input)
	require.NoError(t, err)
	require.NotNil(t, mw)

	// ManifestWork name
	require.Equal(t, "np-001-nodepool-adapter", mw.Name)

	// Labels
	require.Equal(t, "cluster-abc", mw.Labels["hyperfleet.io/cluster-id"])
	require.Equal(t, "np-001", mw.Labels["hyperfleet.io/nodepool-id"])
	require.Equal(t, "nodepool-adapter", mw.Labels["hyperfleet.io/adapter"])
	require.Equal(t, "node-pool", mw.Labels["hyperfleet.io/component"])

	// Generation annotation
	require.Equal(t, "3", mw.Annotations["hyperfleet.io/generation"])

	// Exactly 1 manifest
	require.Len(t, mw.Spec.Workload.Manifests, 1)

	// Unmarshal manifest and check fields
	var nodePool map[string]any
	require.NoError(t, json.Unmarshal(mw.Spec.Workload.Manifests[0].Raw, &nodePool))

	require.Equal(t, "hypershift.openshift.io/v1beta1", nodePool["apiVersion"])
	require.Equal(t, "NodePool", nodePool["kind"])

	meta := nodePool["metadata"].(map[string]any)
	require.Equal(t, "my-nodepool", meta["name"])
	require.Equal(t, "clusters-cluster-abc", meta["namespace"])

	metaLabels := meta["labels"].(map[string]any)
	require.Equal(t, "cluster-abc", metaLabels["hyperfleet.io/cluster-id"])
	require.Equal(t, "np-001", metaLabels["hyperfleet.io/nodepool-id"])
	require.Equal(t, "nodepool-adapter", metaLabels["hyperfleet.io/managed-by"])

	metaAnnotations := meta["annotations"].(map[string]any)
	require.Equal(t, "3", metaAnnotations["hyperfleet.io/generation"])

	spec := nodePool["spec"].(map[string]any)
	require.Equal(t, "my-cluster", spec["clusterName"])
	require.EqualValues(t, 2, spec["replicas"])
	require.Equal(t, "amd64", spec["arch"])

	release := spec["release"].(map[string]any)
	require.Equal(t, "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64", release["image"])

	platform := spec["platform"].(map[string]any)
	require.Equal(t, "GCP", platform["type"])
	gcp := platform["gcp"].(map[string]any)
	require.Equal(t, "n2-standard-8", gcp["machineType"])
	require.Equal(t, "us-central1-b", gcp["zone"])
	require.Equal(t, "my-subnet", gcp["subnet"])
	bootDisk := gcp["bootDisk"].(map[string]any)
	require.EqualValues(t, 200, bootDisk["diskSizeGB"])
	require.Equal(t, "pd-standard", bootDisk["diskType"])

	// deleteOption
	require.NotNil(t, mw.Spec.DeleteOption)
	require.Equal(t, workv1.DeletePropagationPolicyTypeForeground, mw.Spec.DeleteOption.PropagationPolicy)

	// manifestConfigs: single entry with ServerSideApply + 4 feedbackRules
	require.Len(t, mw.Spec.ManifestConfigs, 1)
	cfg := mw.Spec.ManifestConfigs[0]
	require.Equal(t, "hypershift.openshift.io", cfg.ResourceIdentifier.Group)
	require.Equal(t, "nodepools", cfg.ResourceIdentifier.Resource)
	require.Equal(t, "clusters-cluster-abc", cfg.ResourceIdentifier.Namespace)
	require.Equal(t, "my-nodepool", cfg.ResourceIdentifier.Name)
	require.NotNil(t, cfg.UpdateStrategy)
	require.Equal(t, workv1.UpdateStrategyTypeServerSideApply, cfg.UpdateStrategy.Type)
	require.Len(t, cfg.FeedbackRules, 1)
	require.Equal(t, workv1.JSONPathsType, cfg.FeedbackRules[0].Type)
	require.Len(t, cfg.FeedbackRules[0].JsonPaths, 4)

	names := make(map[string]bool)
	for _, jp := range cfg.FeedbackRules[0].JsonPaths {
		names[jp.Name] = true
	}
	require.True(t, names["readyCondition"])
	require.True(t, names["allNodesHealthyCondition"])
	require.True(t, names["replicas"])
	require.True(t, names["version"])
}

func TestBuild_ZoneFallback(t *testing.T) {
	input := Input{
		NodePoolID:         "np-002",
		NodePoolName:       "np-fallback",
		NodePoolGeneration: 1,
		ClusterID:          "cluster-xyz",
		ClusterName:        "cl",
		GCPRegion:          "us-west1",
		Zone:               "", // should fall back
		ReleaseImage:       "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
	}

	mw, err := Build(input)
	require.NoError(t, err)

	var nodePool map[string]any
	require.NoError(t, json.Unmarshal(mw.Spec.Workload.Manifests[0].Raw, &nodePool))

	spec := nodePool["spec"].(map[string]any)
	platform := spec["platform"].(map[string]any)
	gcp := platform["gcp"].(map[string]any)
	require.Equal(t, "us-west1-a", gcp["zone"])
}

func TestBuild_DiskDefaults(t *testing.T) {
	input := Input{
		NodePoolID:         "np-003",
		NodePoolName:       "np-defaults",
		NodePoolGeneration: 1,
		ClusterID:          "cluster-def",
		ClusterName:        "cl",
		GCPRegion:          "us-east1",
		ReleaseImage:       "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
		// DiskSizeGB, DiskType, MachineType intentionally zero/empty
	}

	mw, err := Build(input)
	require.NoError(t, err)

	var nodePool map[string]any
	require.NoError(t, json.Unmarshal(mw.Spec.Workload.Manifests[0].Raw, &nodePool))

	spec := nodePool["spec"].(map[string]any)
	platform := spec["platform"].(map[string]any)
	gcp := platform["gcp"].(map[string]any)

	require.Equal(t, "n2-standard-4", gcp["machineType"])
	bootDisk := gcp["bootDisk"].(map[string]any)
	require.EqualValues(t, 100, bootDisk["diskSizeGB"])
	require.Equal(t, "pd-ssd", bootDisk["diskType"])
}

func TestBuild_ManifestCount(t *testing.T) {
	input := Input{
		NodePoolID:         "np-count",
		NodePoolName:       "np-count",
		NodePoolGeneration: 1,
		ClusterID:          "cid",
		ClusterName:        "cl",
		GCPRegion:          "us-central1",
		ReleaseImage:       "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
	}

	mw, err := Build(input)
	require.NoError(t, err)
	require.Len(t, mw.Spec.Workload.Manifests, 1, "expected exactly 1 manifest (the NodePool CR)")
}
