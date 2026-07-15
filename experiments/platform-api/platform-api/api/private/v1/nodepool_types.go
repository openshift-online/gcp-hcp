package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
type NodePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +orlop:public
	Spec NodePoolSpec `json:"spec,omitempty"`
	// +orlop:public
	Status NodePoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type NodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// +orlop:public
	Items []NodePool `json:"items"`
}

type NodePoolSpec struct {
	// +orlop:public
	// +kubebuilder:validation:Required
	ClusterID string `json:"clusterID"`
	// +orlop:public
	Platform NodePoolPlatformSpec `json:"platform"`
	// +orlop:public
	Release *ClusterReleaseSpec `json:"release,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Minimum=0
	NodeCount *int32 `json:"nodeCount,omitempty"`
	// +orlop:public
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`
}

type NodePoolPlatformSpec struct {
	// +orlop:public
	// +kubebuilder:validation:Enum=GCP
	Type string `json:"type"`
	// +orlop:public
	GCP *GCPNodePoolPlatform `json:"gcp,omitempty"`
}

type GCPNodePoolPlatform struct {
	// +orlop:public
	ProjectID string `json:"projectID,omitempty"`
	// +orlop:public
	Region string `json:"region,omitempty"`
	// +orlop:public
	MachineType string `json:"machineType,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Minimum=10
	DiskSize int32 `json:"diskSize,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Enum=pd-standard;pd-ssd;pd-balanced
	DiskType string `json:"diskType,omitempty"`
	// +orlop:public
	Zones []string `json:"zones,omitempty"`
	// +orlop:public
	Preemptible bool `json:"preemptible,omitempty"`
	// +orlop:public
	Accelerators []AcceleratorSpec `json:"accelerators,omitempty"`
	// +orlop:public
	Labels map[string]string `json:"labels,omitempty"`
	// +orlop:public
	Taints []TaintSpec `json:"taints,omitempty"`
}

type AcceleratorSpec struct {
	// +orlop:public
	Type string `json:"type"`
	// +orlop:public
	// +kubebuilder:validation:Minimum=1
	Count int32 `json:"count"`
}

type TaintSpec struct {
	// +orlop:public
	Key string `json:"key"`
	// +orlop:public
	Value string `json:"value,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
	Effect string `json:"effect"`
}

type AutoscalingSpec struct {
	// +orlop:public
	Enabled bool `json:"enabled,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Minimum=0
	MinNodes int32 `json:"minNodes,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Minimum=1
	MaxNodes int32 `json:"maxNodes,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetCPUUtilization int32 `json:"targetCPUUtilization,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetMemoryUtilization int32 `json:"targetMemoryUtilization,omitempty"`
	// +orlop:public
	ScaleDownDelay string `json:"scaleDownDelay,omitempty"`
	// +orlop:public
	ScaleUpDelay string `json:"scaleUpDelay,omitempty"`
}

// NodePoolStatus is written by controllers only.
type NodePoolStatus struct {
	// +orlop:public
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// VersionResolution is written by the nodepool-vr controller.
	// Not exposed on the public API.
	VersionResolution *VersionResolutionResult `json:"versionResolution,omitempty"`
}

func init() { register(&NodePool{}, &NodePoolList{}) }
