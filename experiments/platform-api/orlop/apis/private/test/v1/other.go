package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
type Other struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +orlop:public
	Spec OtherSpec `json:"spec,omitempty"`
	// +orlop:public
	Status OtherStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type OtherList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// +orlop:public
	Items []Other `json:"items"`
}

type OtherSpec struct {
	// +orlop:public
	PublicField   string `json:"publicField"`
	InternalField string `json:"internalField"`
}

type OtherStatus struct{}

func init() { register(&Other{}, &OtherList{}) }
