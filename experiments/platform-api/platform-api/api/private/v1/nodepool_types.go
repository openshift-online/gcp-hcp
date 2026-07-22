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

// +kubebuilder:validation:XValidation:rule="!has(self.nodeCount) || !has(self.autoscaling)",message="nodeCount and autoscaling are mutually exclusive"
type NodePoolSpec struct {
	// +orlop:public
	// +kubebuilder:validation:Required
	ClusterID string `json:"clusterID"`
	// +orlop:public
	Platform NodePoolPlatformSpec `json:"platform"`
	// +orlop:public
	Release ReleaseSpec `json:"release"`
	// +orlop:public
	// +kubebuilder:validation:Minimum=0
	NodeCount *int32 `json:"nodeCount,omitempty"`
	// +orlop:public
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`
	// +orlop:public
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`
	// +orlop:public
	Taints []TaintSpec `json:"taints,omitempty"`
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
	MachineType string `json:"machineType,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Minimum=20
	DiskSizeGB int64 `json:"diskSizeGB,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Enum=pd-standard;pd-ssd;pd-balanced
	DiskType string `json:"diskType,omitempty"`
	// +orlop:public
	Zone string `json:"zone,omitempty"`
	// +orlop:public
	Subnet string `json:"subnet,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Enum=Standard;Spot;Preemptible
	ProvisioningModel string `json:"provisioningModel,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Enum=MIGRATE;TERMINATE
	OnHostMaintenance string `json:"onHostMaintenance,omitempty"`
	// +orlop:public
	ResourceLabels []GCPResourceLabel `json:"resourceLabels,omitempty"`
	// +orlop:public
	NetworkTags []string `json:"networkTags,omitempty"`
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

// +kubebuilder:validation:XValidation:rule="self.max >= self.min",message="max must be greater than or equal to min"
type AutoscalingSpec struct {
	// +orlop:public
	// +kubebuilder:validation:Minimum=0
	Min *int32 `json:"min,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Minimum=1
	Max int32 `json:"max"`
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
