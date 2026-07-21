package test

import (
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/thetechnick/orlop/apis/private/test/v1"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme.
var AddToSchemes runtime.SchemeBuilder = runtime.SchemeBuilder{
	v1.SchemeBuilder.AddToScheme,
}

// AddToScheme adds all core Resources to the Scheme.
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
