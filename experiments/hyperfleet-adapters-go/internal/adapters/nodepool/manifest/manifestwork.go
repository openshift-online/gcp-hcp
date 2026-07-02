// Package manifest provides the ManifestWork builder for the nodepool adapter.
package manifest

import (
	"encoding/json"
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
)

const (
	adapterName   = "nodepool-adapter"
	componentName = "node-pool"

	// DefaultDiskSizeGB is the default disk size in GB for GCP node pool boot disks.
	DefaultDiskSizeGB = int32(100)
	// DefaultDiskType is the default disk type for GCP node pool boot disks.
	DefaultDiskType = "pd-ssd"
	// DefaultMachineType is the default GCP machine type for node pool instances.
	DefaultMachineType = "n2-standard-4"

	defaultReplicas = int32(1)

	hypershiftGroup   = "hypershift.openshift.io"
	nodepoolResource  = "nodepools"
	nodepoolAPIVersion = "hypershift.openshift.io/v1beta1"
	nodepoolKind      = "NodePool"
)

// Input holds all parameters for building the NodePool ManifestWork.
type Input struct {
	NodePoolID         string
	NodePoolName       string
	NodePoolGeneration int64
	ClusterID          string
	ClusterName        string
	Replicas           int32
	MachineType        string // e.g. "n2-standard-4"
	GCPRegion          string
	Zone               string // optional; falls back to GCPRegion+"-a"
	GCPSubnet          string
	DiskSizeGB         int32  // default: 100
	DiskType           string // default: "pd-ssd"
	ReleaseImage       string
}

// Build constructs a *workv1.ManifestWork for a NodePool.
func Build(input Input) (*workv1.ManifestWork, error) {
	// Apply defaults
	zone := input.Zone
	if zone == "" {
		zone = input.GCPRegion + "-a"
	}

	diskSizeGB := input.DiskSizeGB
	if diskSizeGB == 0 {
		diskSizeGB = DefaultDiskSizeGB
	}

	diskType := input.DiskType
	if diskType == "" {
		diskType = DefaultDiskType
	}

	machineType := input.MachineType
	if machineType == "" {
		machineType = DefaultMachineType
	}

	replicas := input.Replicas
	if replicas == 0 {
		replicas = defaultReplicas
	}

	genStr := strconv.FormatInt(input.NodePoolGeneration, 10)
	namespace := fmt.Sprintf("clusters-%s", input.ClusterID)
	mwName := fmt.Sprintf("%s-%s", input.NodePoolID, adapterName)

	// Build the NodePool manifest as map[string]any
	nodePoolManifest := map[string]any{
		"apiVersion": nodepoolAPIVersion,
		"kind":       nodepoolKind,
		"metadata": map[string]any{
			"name":      input.NodePoolName,
			"namespace": namespace,
			"labels": map[string]any{
				"hyperfleet.io/cluster-id":  input.ClusterID,
				"hyperfleet.io/nodepool-id": input.NodePoolID,
				"hyperfleet.io/managed-by": adapterName,
			},
			"annotations": map[string]any{
				"hyperfleet.io/generation": genStr,
			},
		},
		"spec": map[string]any{
			"clusterName": input.ClusterName,
			"replicas":    replicas,
			"release": map[string]any{
				"image": input.ReleaseImage,
			},
			"arch": "amd64",
			"management": map[string]any{
				"autoRepair":  true,
				"upgradeType": "Replace",
				"replace": map[string]any{
					"strategy": "RollingUpdate",
					"rollingUpdate": map[string]any{
						"maxSurge":       1,
						"maxUnavailable": 0,
					},
				},
			},
			"platform": map[string]any{
				"type": "GCP",
				"gcp": map[string]any{
					"machineType": machineType,
					"zone":        zone,
					"subnet":      input.GCPSubnet,
					"bootDisk": map[string]any{
						"diskSizeGB": diskSizeGB,
						"diskType":   diskType,
					},
				},
			},
		},
	}

	rawBytes, err := json.Marshal(nodePoolManifest)
	if err != nil {
		return nil, fmt.Errorf("nodepool manifest: marshal NodePool: %w", err)
	}

	mw := &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "work.open-cluster-management.io/v1",
			Kind:       "ManifestWork",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: mwName,
			Labels: map[string]string{
				"hyperfleet.io/cluster-id":  input.ClusterID,
				"hyperfleet.io/nodepool-id": input.NodePoolID,
				"hyperfleet.io/adapter":     adapterName,
				"hyperfleet.io/component":   componentName,
			},
			Annotations: map[string]string{
				"hyperfleet.io/generation": genStr,
			},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					{
						RawExtension: runtime.RawExtension{Raw: rawBytes},
					},
				},
			},
			DeleteOption: &workv1.DeleteOption{
				PropagationPolicy: workv1.DeletePropagationPolicyTypeForeground,
			},
			ManifestConfigs: []workv1.ManifestConfigOption{
				{
					ResourceIdentifier: workv1.ResourceIdentifier{
						Group:     hypershiftGroup,
						Resource:  nodepoolResource,
						Namespace: namespace,
						Name:      input.NodePoolName,
					},
					UpdateStrategy: &workv1.UpdateStrategy{
						Type: workv1.UpdateStrategyTypeServerSideApply,
					},
					FeedbackRules: []workv1.FeedbackRule{
						{
							Type: workv1.JSONPathsType,
							JsonPaths: []workv1.JsonPath{
								{
									Name: "readyCondition",
									Path: `.status.conditions[?(@.type=="Ready")].status`,
								},
								{
									Name: "allNodesHealthyCondition",
									Path: `.status.conditions[?(@.type=="AllNodesHealthy")].status`,
								},
								{
									Name: "replicas",
									Path: ".status.replicas",
								},
								{
									Name: "version",
									Path: ".status.version",
								},
							},
						},
					},
				},
			},
		},
	}

	return mw, nil
}
