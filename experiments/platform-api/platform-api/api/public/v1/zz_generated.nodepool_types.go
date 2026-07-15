package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
type NodePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NodePoolSpec `json:"spec,omitempty"`

	Status NodePoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type NodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []NodePool `json:"items"`
}

type NodePoolSpec struct {

	// +kubebuilder:validation:Required
	ClusterID string `json:"clusterID"`

	Platform NodePoolPlatformSpec `json:"platform"`

	Release *ClusterReleaseSpec `json:"release,omitempty"`

	// +kubebuilder:validation:Minimum=0
	NodeCount *int32 `json:"nodeCount,omitempty"`

	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`
}

type NodePoolPlatformSpec struct {

	// +kubebuilder:validation:Enum=GCP
	Type string `json:"type"`

	GCP *GCPNodePoolPlatform `json:"gcp,omitempty"`
}

type GCPNodePoolPlatform struct {
	ProjectID string `json:"projectID,omitempty"`

	Region string `json:"region,omitempty"`

	MachineType string `json:"machineType,omitempty"`

	// +kubebuilder:validation:Minimum=10
	DiskSize int32 `json:"diskSize,omitempty"`

	// +kubebuilder:validation:Enum=pd-standard;pd-ssd;pd-balanced
	DiskType string `json:"diskType,omitempty"`

	Zones []string `json:"zones,omitempty"`

	Preemptible bool `json:"preemptible,omitempty"`

	Accelerators []AcceleratorSpec `json:"accelerators,omitempty"`

	Labels map[string]string `json:"labels,omitempty"`

	Taints []TaintSpec `json:"taints,omitempty"`
}

type AcceleratorSpec struct {
	Type string `json:"type"`

	// +kubebuilder:validation:Minimum=1
	Count int32 `json:"count"`
}

type TaintSpec struct {
	Key string `json:"key"`

	Value string `json:"value,omitempty"`

	// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
	Effect string `json:"effect"`
}

type AutoscalingSpec struct {
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Minimum=0
	MinNodes int32 `json:"minNodes,omitempty"`

	// +kubebuilder:validation:Minimum=1
	MaxNodes int32 `json:"maxNodes,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetCPUUtilization int32 `json:"targetCPUUtilization,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetMemoryUtilization int32 `json:"targetMemoryUtilization,omitempty"`

	ScaleDownDelay string `json:"scaleDownDelay,omitempty"`

	ScaleUpDelay string `json:"scaleUpDelay,omitempty"`
}

// NodePoolStatus is written by controllers only.
type NodePoolStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func init() { register(&NodePool{}, &NodePoolList{}) }
