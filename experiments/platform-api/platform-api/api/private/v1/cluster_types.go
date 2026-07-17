package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +orlop:public
	Spec ClusterSpec `json:"spec,omitempty"`
	// +orlop:public
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// +orlop:public
	Items []Cluster `json:"items"`
}

// ClusterSpec is user-defined input only.
type ClusterSpec struct {
	// +orlop:public
	InfraID string `json:"infraID,omitempty"`
	// +orlop:public
	IssuerURL string `json:"issuerURL,omitempty"`
	// +orlop:public
	Platform ClusterPlatformSpec `json:"platform"`
	// +orlop:public
	Release *ClusterReleaseSpec `json:"release,omitempty"`
	// +orlop:public
	Networking *NetworkingSpec `json:"networking,omitempty"`
	// +orlop:public
	DNS *DNSSpec `json:"dns,omitempty"`
}

type ClusterPlatformSpec struct {
	// +orlop:public
	// +kubebuilder:validation:Enum=GCP
	Type string `json:"type"`
	// +orlop:public
	GCP *GCPClusterPlatform `json:"gcp,omitempty"`
}

type GCPClusterPlatform struct {
	// +orlop:public
	ProjectID string `json:"projectID,omitempty"`
	// +orlop:public
	Region string `json:"region,omitempty"`
	// +orlop:public
	Network string `json:"network,omitempty"`
	// +orlop:public
	Subnet string `json:"subnet,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Enum=PublicAndPrivate;Private
	EndpointAccess string `json:"endpointAccess,omitempty"`
	// +orlop:public
	// +kubebuilder:validation:Required
	WorkloadIdentity WorkloadIdentitySpec `json:"workloadIdentity"`
	// +orlop:public
	ResourceLabels []GCPResourceLabel `json:"resourceLabels,omitempty"`
}

type WorkloadIdentitySpec struct {
	// +orlop:public
	PoolID string `json:"poolID,omitempty"`
	// +orlop:public
	ProjectNumber string `json:"projectNumber,omitempty"`
	// +orlop:public
	ProviderID string `json:"providerID,omitempty"`
	// +orlop:public
	ServiceAccountsRef *ServiceAccountsRef `json:"serviceAccountsRef,omitempty"`
}

type ServiceAccountsRef struct {
	// +orlop:public
	NodePoolEmail string `json:"nodePoolEmail,omitempty"`
	// +orlop:public
	ControlPlaneEmail string `json:"controlPlaneEmail,omitempty"`
	// +orlop:public
	CloudControllerEmail string `json:"cloudControllerEmail,omitempty"`
	// +orlop:public
	StorageEmail string `json:"storageEmail,omitempty"`
	// +orlop:public
	ImageRegistryEmail string `json:"imageRegistryEmail,omitempty"`
	// +orlop:public
	NetworkEmail string `json:"networkEmail,omitempty"`
}

// GCPResourceLabel is a label applied to GCP resources created for the cluster.
type GCPResourceLabel struct {
	// +orlop:public
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Key string `json:"key"`
	// +orlop:public
	// +kubebuilder:validation:MaxLength=63
	Value string `json:"value"`
}

// ClusterReleaseSpec defines the target OCP release version.
// The version-resolution adapter resolves Version+ChannelGroup to a release image pullspec.
type ClusterReleaseSpec struct {
	// +orlop:public
	Version string `json:"version,omitempty"`
	// +orlop:public
	ChannelGroup string `json:"channelGroup,omitempty"`
}

type NetworkingSpec struct {
	// +orlop:public
	ClusterNetwork []ClusterNetworkEntry `json:"clusterNetwork,omitempty"`
	// +orlop:public
	ServiceNetwork []string `json:"serviceNetwork,omitempty"`
}

type ClusterNetworkEntry struct {
	// +orlop:public
	CIDR string `json:"cidr,omitempty"`
	// +orlop:public
	HostPrefix int32 `json:"hostPrefix,omitempty"`
}

type DNSSpec struct {
	// +orlop:public
	BaseDomain string `json:"baseDomain,omitempty"`
}

// ClusterStatus is written by controllers only.
type ClusterStatus struct {
	// +orlop:public
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// PlacementResult is written by the placement controller.
	// +orlop:public
	PlacementResult *PlacementResult `json:"placementResult,omitempty"`

	// HostedClusterResult is written by the hc-adapter.
	// +orlop:public
	HostedClusterResult *HostedClusterResult `json:"hostedClusterResult,omitempty"`

	// VersionResolution is written by the version-resolution controller.
	// Not exposed on the public API.
	VersionResolution *VersionResolutionResult `json:"versionResolution,omitempty"`
}

// PlacementResult holds the placement controller's output.
type PlacementResult struct {
	ManagementClusterName string `json:"managementClusterName,omitempty"`
	BaseDomain            string `json:"baseDomain,omitempty"`
}

// VersionResolutionResult holds the VR controller's output.
type VersionResolutionResult struct {
	ReleaseImage   string `json:"releaseImage,omitempty"`
	ReleaseVersion string `json:"releaseVersion,omitempty"`
	ReleaseChannel string `json:"releaseChannel,omitempty"`
}

// HostedClusterResult holds the hc-adapter's output from ManifestWork status feedback.
// This field is read-only — populated by the hc-adapter only.
type HostedClusterResult struct {
	// +orlop:public
	APIEndpoint string `json:"apiEndpoint,omitempty"`
	// +orlop:public
	Version string `json:"version,omitempty"`
}

func init() { register(&Cluster{}, &ClusterList{}) }
