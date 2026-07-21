/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

// HostedClusterSpec defines the desired state of a HyperShift HostedCluster.
// These types are a curated subset of the upstream HyperShift API, copied
// here to avoid a direct Go module dependency on the HyperShift operator.
type HostedClusterSpec struct {
	// release specifies the OCP release image for the hosted cluster.
	// +required
	Release ReleaseSpec `json:"release"`

	// infraID is the unique infrastructure identifier for this cluster.
	// +required
	InfraID string `json:"infraID"`

	// platform specifies the underlying infrastructure provider for the cluster.
	// +required
	// +orlop:public
	Platform PlatformSpec `json:"platform" openapi:"hidden"`

	// networking defines the networking configuration for the cluster.
	// +optional
	Networking *ClusterNetworkingSpec `json:"networking,omitempty" openapi:"hidden"`

	// etcd defines the etcd configuration for the hosted control plane.
	// +optional
	Etcd *EtcdSpec `json:"etcd,omitempty" openapi:"hidden"`

	// services defines the service publishing strategy for control plane services.
	// +optional
	Services []ServicePublishingStrategyMapping `json:"services,omitempty" openapi:"hidden"`

	// channel is the Cincinnati release channel (e.g. "stable-4.16").
	// +optional
	// +orlop:public
	Channel string `json:"channel,omitempty" openapi:"hidden"`

	// controllerAvailabilityPolicy specifies the availability policy for control plane controllers.
	// +optional
	ControllerAvailabilityPolicy string `json:"controllerAvailabilityPolicy,omitempty" openapi:"hidden"`

	// pullSecret references the Secret containing the pull secret for the cluster.
	// +optional
	PullSecret *SecretReference `json:"pullSecret,omitempty" openapi:"hidden"`

	// serviceAccountSigningKey references the Secret containing the service account signing key.
	// +optional
	ServiceAccountSigningKey *SecretReference `json:"serviceAccountSigningKey,omitempty" openapi:"hidden"`

	// clusterID is the unique cluster identifier passed to the HostedCluster CR.
	// +optional
	ClusterID string `json:"clusterID,omitempty" openapi:"hidden"`

	// capabilities defines which cluster capabilities are enabled or disabled.
	// +optional
	Capabilities *CapabilitiesSpec `json:"capabilities,omitempty" openapi:"hidden"`

	// configuration defines cluster-level configuration (authentication, API server, etc.).
	// +optional
	// +orlop:public
	Configuration *ClusterConfiguration `json:"configuration,omitempty" openapi:"hidden"`
}

// ReleaseSpec defines the target OCP release.
type ReleaseSpec struct {
	// image is the release image pullspec (e.g. quay.io/openshift-release-dev/ocp-release:4.16.3-x86_64).
	// +required
	Image string `json:"image"`
}

// PlatformSpec specifies the underlying infrastructure provider configuration.
type PlatformSpec struct {
	// type is the infrastructure provider type (e.g. "None", "GCP", "AWS").
	// +required
	// +orlop:public
	Type string `json:"type"`
}

// ClusterNetworkingSpec defines the networking configuration for the cluster.
type ClusterNetworkingSpec struct {
	// clusterNetwork is the list of IP address pools for pod IPs.
	// +optional
	ClusterNetwork []NetworkRange `json:"clusterNetwork,omitempty"`

	// serviceNetwork is the list of IP address pools for service IPs.
	// +optional
	ServiceNetwork []NetworkRange `json:"serviceNetwork,omitempty"`

	// networkType is the CNI plugin type (e.g. "OVNKubernetes", "OpenShiftSDN").
	// +optional
	NetworkType string `json:"networkType,omitempty"`
}

// NetworkRange defines a network CIDR range.
type NetworkRange struct {
	// cidr is the IP address range in CIDR notation (e.g. 10.132.0.0/14).
	// +required
	CIDR string `json:"cidr"`
}

// EtcdSpec defines the etcd configuration.
type EtcdSpec struct {
	// managementType defines how etcd is managed (e.g. "Managed", "Unmanaged").
	// +required
	ManagementType string `json:"managementType"`

	// managed defines configuration when managementType is "Managed".
	// +optional
	Managed *ManagedEtcdSpec `json:"managed,omitempty"`
}

// ManagedEtcdSpec defines configuration for managed etcd.
type ManagedEtcdSpec struct {
	// storage defines the storage configuration for etcd.
	// +required
	Storage ManagedEtcdStorageSpec `json:"storage"`
}

// ManagedEtcdStorageSpec defines etcd storage configuration.
type ManagedEtcdStorageSpec struct {
	// type is the storage backend type (e.g. "PersistentVolume").
	// +required
	Type string `json:"type"`
}

// ServicePublishingStrategyMapping defines how a control plane service is published.
type ServicePublishingStrategyMapping struct {
	// service is the name of the service (e.g. "APIServer", "OAuthServer", "Konnectivity", "Ignition").
	// +required
	Service string `json:"service"`

	// servicePublishingStrategy defines how the service is made available.
	// +required
	ServicePublishingStrategy ServicePublishingStrategy `json:"servicePublishingStrategy"`
}

// ServicePublishingStrategy specifies how to publish a service.
type ServicePublishingStrategy struct {
	// type is the publishing strategy (e.g. "LoadBalancer", "Route", "NodePort").
	// +required
	Type string `json:"type"`
}

// SecretReference is a reference to a Secret by name.
type SecretReference struct {
	// name is the name of the Secret.
	// +required
	Name string `json:"name"`
}

// CapabilitiesSpec defines which cluster capabilities are enabled or disabled.
type CapabilitiesSpec struct {
	// disabled is the list of cluster capabilities to disable.
	// +optional
	Disabled []string `json:"disabled,omitempty"`
}

// ClusterConfiguration defines cluster-level configuration.
type ClusterConfiguration struct {
	// authentication defines the authentication configuration for the cluster.
	// +optional
	// +orlop:public
	Authentication *AuthenticationConfig `json:"authentication,omitempty"`
}

// AuthenticationConfig defines authentication settings.
type AuthenticationConfig struct {
	// type is the authentication type (e.g. "OIDC").
	// +optional
	// +orlop:public
	Type string `json:"type,omitempty"`

	// oidcProviders defines the OIDC identity providers for the cluster.
	// +optional
	// +orlop:public
	OIDCProviders []OIDCProvider `json:"oidcProviders,omitempty"`
}

// OIDCProvider defines an OIDC identity provider configuration.
type OIDCProvider struct {
	// name is the name of the OIDC provider.
	// +required
	// +orlop:public
	Name string `json:"name"`

	// issuer defines the OIDC issuer configuration.
	// +required
	// +orlop:public
	Issuer OIDCIssuer `json:"issuer"`

	// claimMappings defines how OIDC claims are mapped to Kubernetes identities.
	// +required
	// +orlop:public
	ClaimMappings ClaimMappings `json:"claimMappings"`
}

// OIDCIssuer defines the OIDC issuer endpoint.
type OIDCIssuer struct {
	// issuerURL is the URL of the OIDC issuer.
	// +required
	IssuerURL string `json:"issuerURL"`

	// audiences is the list of valid audiences for tokens issued by this provider.
	// +optional
	Audiences []string `json:"audiences,omitempty"`
}

// ClaimMappings defines how OIDC claims map to Kubernetes identities.
type ClaimMappings struct {
	// username defines the claim used for the Kubernetes username.
	// +required
	Username ClaimMapping `json:"username"`

	// groups defines the claim used for Kubernetes group membership.
	// +optional
	Groups *ClaimMapping `json:"groups,omitempty"`
}

// ClaimMapping defines how a single OIDC claim is mapped.
type ClaimMapping struct {
	// claim is the name of the OIDC claim.
	// +required
	Claim string `json:"claim"`

	// prefix is prepended to claim values.
	// +optional
	Prefix string `json:"prefix,omitempty"`
}
