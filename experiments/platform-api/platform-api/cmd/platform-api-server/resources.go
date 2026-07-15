package main

import (
	privatev1 "github.com/thetechnick/orlop-gcp-hcp/api/private/v1"
	privatev1alpha1 "github.com/thetechnick/orlop-gcp-hcp/api/private/v1alpha1"
	privatev1beta1hs "github.com/thetechnick/orlop-gcp-hcp/api/private/v1beta1hs"
	publicv1 "github.com/thetechnick/orlop-gcp-hcp/api/public/v1"
	publicv1alpha1 "github.com/thetechnick/orlop-gcp-hcp/api/public/v1alpha1"
	"github.com/thetechnick/orlop/pkg/apiserver/types"
	"k8s.io/apimachinery/pkg/runtime"
)

// getPrivateResources returns the resource definitions for the private API.
// Uses generated ResourceInfo from the private API package.
func getPrivateResources() []types.ResourceInfo {
	resources := append(privatev1.GetResourceInfos(), privatev1alpha1.GetResourceInfos()...)
	resources = append(resources, privatev1beta1hs.GetResourceInfos()...)
	// Wire up NodePool as a child of Cluster for nested route support.
	for i, res := range resources {
		if res.GVK.Kind == "NodePool" && res.GVK.Group == privatev1.GroupVersion.Group {
			resources[i].ParentResource = &types.ParentResourceInfo{
				Plural:  "clusters",
				IDField: "spec.clusterID",
			}
		}
	}
	return resources
}

// getPublicResources returns the resource definitions for the public API.
// Uses generated ResourceInfo from the public API package.
func getPublicResources() []types.ResourceInfo {
	return append(publicv1.GetResourceInfos(), publicv1alpha1.GetResourceInfos()...)
}

// getPrivateScheme creates and returns a runtime.Scheme with private API types registered.
func getPrivateScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	privatev1.AddToScheme(scheme)
	privatev1alpha1.AddToScheme(scheme)
	privatev1beta1hs.AddToScheme(scheme)
	return scheme
}

// getPublicScheme creates and returns a runtime.Scheme with public API types registered.
func getPublicScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	publicv1.AddToScheme(scheme)
	publicv1alpha1.AddToScheme(scheme)
	return scheme
}
