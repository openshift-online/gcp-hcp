// +kubebuilder:object:generate=true
// +groupName=hyperkube
// +orlop:public
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "hyperkube", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder runtime.SchemeBuilder

	// localSchemeBuilder is used for registration of conversion functions
	localSchemeBuilder = &SchemeBuilder

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme

	// SchemeGroupVersion is group version used to register these objects
	//
	// Deprecated: use GroupVersion instead.
	SchemeGroupVersion = GroupVersion
)

func register(objs ...runtime.Object) {
	SchemeBuilder.Register(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(GroupVersion, objs...)
		metav1.AddToGroupVersion(scheme, GroupVersion)
		return nil
	})
}
