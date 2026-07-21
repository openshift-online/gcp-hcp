package main

import (
	privatev1alpha1 "github.com/thetechnick/orlop-gcp-hcp/api/private/v1alpha1"
	privatev1beta1hs "github.com/thetechnick/orlop-gcp-hcp/api/private/v1beta1hs"
	publicv1alpha1 "github.com/thetechnick/orlop-gcp-hcp/api/public/v1alpha1"
	"github.com/thetechnick/orlop/pkg/apiserver/types"
	"k8s.io/apimachinery/pkg/runtime"
)

// getPrivateResources returns the resource definitions for the private API.
// Uses generated ResourceInfo from the private API package.
func getPrivateResources() []types.ResourceInfo {
	return append(privatev1alpha1.GetResourceInfos(), privatev1beta1hs.GetResourceInfos()...)
}

// getPublicResources returns the resource definitions for the public API.
// Uses generated ResourceInfo from the public API package.
func getPublicResources() []types.ResourceInfo {
	return publicv1alpha1.GetResourceInfos()
}

// getPrivateScheme creates and returns a runtime.Scheme with private API types registered.
func getPrivateScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	privatev1alpha1.AddToScheme(scheme)
	privatev1beta1hs.AddToScheme(scheme)
	return scheme
}

// getPublicScheme creates and returns a runtime.Scheme with public API types registered.
func getPublicScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	publicv1alpha1.AddToScheme(scheme)
	return scheme
}
