package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterSpec `json:"spec,omitempty"`

	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Cluster `json:"items"`
}

// ClusterSpec is user-defined input only.
type ClusterSpec struct {
	InfraID string `json:"infraID,omitempty"`

	IssuerURL string `json:"issuerURL,omitempty"`

	Platform ClusterPlatformSpec `json:"platform"`

	Release ReleaseSpec `json:"release"`

	Networking NetworkingSpec `json:"networking"`

	DNS *DNSSpec `json:"dns,omitempty"`
}

type ClusterPlatformSpec struct {

	// +kubebuilder:validation:Enum=GCP
	Type string `json:"type"`

	GCP *GCPClusterPlatform `json:"gcp,omitempty"`
}

type GCPClusterPlatform struct {
	ProjectID string `json:"projectID,omitempty"`

	Region string `json:"region,omitempty"`

	Network string `json:"network,omitempty"`

	Subnet string `json:"subnet,omitempty"`

	// +kubebuilder:validation:Enum=PublicAndPrivate;Private
	EndpointAccess string `json:"endpointAccess,omitempty"`

	// +kubebuilder:validation:Required
	WorkloadIdentity WorkloadIdentitySpec `json:"workloadIdentity"`

	ResourceLabels []GCPResourceLabel `json:"resourceLabels,omitempty"`
}

type WorkloadIdentitySpec struct {
	PoolID string `json:"poolID,omitempty"`

	ProjectNumber string `json:"projectNumber,omitempty"`

	ProviderID string `json:"providerID,omitempty"`

	ServiceAccountsRef *ServiceAccountsRef `json:"serviceAccountsRef,omitempty"`
}

type ServiceAccountsRef struct {
	NodePoolEmail string `json:"nodePoolEmail,omitempty"`

	ControlPlaneEmail string `json:"controlPlaneEmail,omitempty"`

	CloudControllerEmail string `json:"cloudControllerEmail,omitempty"`

	StorageEmail string `json:"storageEmail,omitempty"`

	ImageRegistryEmail string `json:"imageRegistryEmail,omitempty"`

	NetworkEmail string `json:"networkEmail,omitempty"`
}

// GCPResourceLabel is a label applied to GCP resources created for the cluster.
type GCPResourceLabel struct {

	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Key string `json:"key"`

	// +kubebuilder:validation:MaxLength=63
	Value string `json:"value"`
}

// ReleaseSpec defines the target OCP release version.
// The version-resolution adapter resolves Version+ChannelGroup to a release image pullspec.
type ReleaseSpec struct {
	Version string `json:"version,omitempty"`

	ChannelGroup string `json:"channelGroup,omitempty"`
}

type NetworkingSpec struct {
	MachineNetwork []MachineNetworkEntry `json:"machineNetwork,omitempty"`

	ClusterNetwork []ClusterNetworkEntry `json:"clusterNetwork,omitempty"`

	ServiceNetwork []string `json:"serviceNetwork,omitempty"`

	// +kubebuilder:validation:Enum=OVNKubernetes;Other
	// +kubebuilder:default=OVNKubernetes
	NetworkType string `json:"networkType,omitempty"`
}

type MachineNetworkEntry struct {
	CIDR string `json:"cidr"`
}

type ClusterNetworkEntry struct {
	CIDR string `json:"cidr,omitempty"`

	HostPrefix int32 `json:"hostPrefix,omitempty"`
}

type DNSSpec struct {
	BaseDomain string `json:"baseDomain,omitempty"`
}

// ClusterStatus is written by controllers only.
type ClusterStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// PlacementResult is written by the placement controller.

	PlacementResult *PlacementResult `json:"placementResult,omitempty"`

	// HostedClusterResult is written by the hc-adapter.

	HostedClusterResult *HostedClusterResult `json:"hostedClusterResult,omitempty"`
}

// PlacementResult holds the placement controller's output.
type PlacementResult struct {
}

// HostedClusterResult holds the hc-adapter's output from ManifestWork status feedback.
// This field is read-only — populated by the hc-adapter only.
type HostedClusterResult struct {
	APIEndpoint string `json:"apiEndpoint,omitempty"`

	Version string `json:"version,omitempty"`
}

func init() { register(&Cluster{}, &ClusterList{}) }
