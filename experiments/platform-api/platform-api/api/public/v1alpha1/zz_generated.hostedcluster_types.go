package v1alpha1

// HostedClusterSpec defines the desired state of a HyperShift HostedCluster.
// These types are a curated subset of the upstream HyperShift API, copied
// here to avoid a direct Go module dependency on the HyperShift operator.
type HostedClusterSpec struct {

	// platform specifies the underlying infrastructure provider for the cluster.
	// +required

	Platform PlatformSpec `json:"platform" openapi:"hidden"`

	// channel is the Cincinnati release channel (e.g. "stable-4.16").
	// +optional

	Channel string `json:"channel,omitempty" openapi:"hidden"`

	// configuration defines cluster-level configuration (authentication, API server, etc.).
	// +optional

	Configuration *ClusterConfiguration `json:"configuration,omitempty" openapi:"hidden"`
}

// ReleaseSpec defines the target OCP release.
type ReleaseSpec struct {
}

// PlatformSpec specifies the underlying infrastructure provider configuration.
type PlatformSpec struct {
	// type is the infrastructure provider type (e.g. "None", "GCP", "AWS").
	// +required

	Type string `json:"type"`
}

// ClusterConfiguration defines cluster-level configuration.
type ClusterConfiguration struct {
	// authentication defines the authentication configuration for the cluster.
	// +optional

	Authentication *AuthenticationConfig `json:"authentication,omitempty"`
}

// AuthenticationConfig defines authentication settings.
type AuthenticationConfig struct {
	// type is the authentication type (e.g. "OIDC").
	// +optional

	Type string `json:"type,omitempty"`

	// oidcProviders defines the OIDC identity providers for the cluster.
	// +optional

	OIDCProviders []OIDCProvider `json:"oidcProviders,omitempty"`
}

// OIDCProvider defines an OIDC identity provider configuration.
type OIDCProvider struct {
	// name is the name of the OIDC provider.
	// +required

	Name string `json:"name"`

	// issuer defines the OIDC issuer configuration.
	// +required

	Issuer OIDCIssuer `json:"issuer"`

	// claimMappings defines how OIDC claims are mapped to Kubernetes identities.
	// +required

	ClaimMappings ClaimMappings `json:"claimMappings"`
}

// OIDCIssuer defines the OIDC issuer endpoint.
type OIDCIssuer struct {
}

// ClaimMappings defines how OIDC claims map to Kubernetes identities.
type ClaimMappings struct {
}
