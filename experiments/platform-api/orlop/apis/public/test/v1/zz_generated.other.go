package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
type Other struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec OtherSpec `json:"spec,omitempty"`

	Status OtherStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type OtherList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Other `json:"items"`
}

type OtherSpec struct {
	PublicField string `json:"publicField"`
}

type OtherStatus struct {
}

func init() { register(&Other{}, &OtherList{}) }
