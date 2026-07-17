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

// +kubebuilder:validation:XValidation:rule="!has(self.nodeCount) || !has(self.autoscaling)",message="nodeCount and autoscaling are mutually exclusive"
type NodePoolSpec struct {

	// +kubebuilder:validation:Required
	ClusterID string `json:"clusterID"`

	Platform NodePoolPlatformSpec `json:"platform"`

	Release ReleaseSpec `json:"release"`

	// +kubebuilder:validation:Minimum=0
	NodeCount *int32 `json:"nodeCount,omitempty"`

	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	NodeLabels map[string]string `json:"nodeLabels,omitempty"`

	Taints []TaintSpec `json:"taints,omitempty"`
}

type NodePoolPlatformSpec struct {

	// +kubebuilder:validation:Enum=GCP
	Type string `json:"type"`

	GCP *GCPNodePoolPlatform `json:"gcp,omitempty"`
}

type GCPNodePoolPlatform struct {
	MachineType string `json:"machineType,omitempty"`

	// +kubebuilder:validation:Minimum=20
	DiskSizeGB int64 `json:"diskSizeGB,omitempty"`

	// +kubebuilder:validation:Enum=pd-standard;pd-ssd;pd-balanced
	DiskType string `json:"diskType,omitempty"`

	Zone string `json:"zone,omitempty"`

	Subnet string `json:"subnet,omitempty"`

	// +kubebuilder:validation:Enum=Standard;Spot;Preemptible
	ProvisioningModel string `json:"provisioningModel,omitempty"`

	// +kubebuilder:validation:Enum=MIGRATE;TERMINATE
	OnHostMaintenance string `json:"onHostMaintenance,omitempty"`

	ResourceLabels []GCPResourceLabel `json:"resourceLabels,omitempty"`

	NetworkTags []string `json:"networkTags,omitempty"`
}

type TaintSpec struct {
	Key string `json:"key"`

	Value string `json:"value,omitempty"`

	// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
	Effect string `json:"effect"`
}

type AutoscalingSpec struct {

	// +kubebuilder:validation:Minimum=0
	Min *int32 `json:"min,omitempty"`

	// +kubebuilder:validation:Minimum=1
	Max int32 `json:"max"`
}

// NodePoolStatus is written by controllers only.
type NodePoolStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func init() { register(&NodePool{}, &NodePoolList{}) }
