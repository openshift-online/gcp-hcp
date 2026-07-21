package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
type Object struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ObjectSpec `json:"spec,omitempty"`

	Status ObjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ObjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Object `json:"items"`
}

type ObjectSpec struct {
	PublicField string `json:"publicField"`

	Nested ObjectNested `json:"nested"`

	// +kubebuilder:default="default-value"
	DefaultField string `json:"defaultField,omitempty"`
}

type ObjectNested struct {
	PublicField string `json:"publicField"`
}

type ObjectStatus struct {
	Conditions []string `json:"conditions,omitempty"`
}

func init() { register(&Object{}, &ObjectList{}) }
