package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ManagedHostedCluster is the Schema for the managedhostedclusters API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type ManagedHostedCluster struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ManagedHostedCluster
	// +required

	Spec ManagedHostedClusterSpec `json:"spec"`

	// status defines the observed state of ManagedHostedCluster
	// +optional

	Status ManagedHostedClusterStatus `json:"status,omitzero"`
}

type ManagedHostedClusterSpec struct {

	// HostedCluster defines HyperShift HostedCluster configuration options.

	HostedCluster HostedClusterSpec `json:"hostedCluster"`
}

type ManagedHostedClusterStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ManagedHostedClusterList contains a list of ManagedHostedCluster
// +kubebuilder:object:root=true
type ManagedHostedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`

	Items []ManagedHostedCluster `json:"items"`
}

func init() { register(&ManagedHostedCluster{}, &ManagedHostedClusterList{}) }
