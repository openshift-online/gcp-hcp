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

	Release *ClusterReleaseSpec `json:"release,omitempty"`

	Networking *NetworkingSpec `json:"networking,omitempty"`

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

	Subnets []SubnetSpec `json:"subnets,omitempty"`

	// +kubebuilder:validation:Enum=PublicAndPrivate;Private;Public
	EndpointAccess string `json:"endpointAccess,omitempty"`

	WorkloadIdentity *WorkloadIdentitySpec `json:"workloadIdentity,omitempty"`
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

type SubnetSpec struct {
	ID string `json:"id"`

	Name string `json:"name"`

	CIDR string `json:"cidr"`

	Role string `json:"role"`
}

// ClusterReleaseSpec defines the target OCP release version.
// The version-resolution adapter resolves Version+ChannelGroup to a release image pullspec.
type ClusterReleaseSpec struct {
	Version string `json:"version,omitempty"`

	ChannelGroup string `json:"channelGroup,omitempty"`
}

type NetworkingSpec struct {
	ClusterNetwork []ClusterNetworkEntry `json:"clusterNetwork,omitempty"`

	ServiceNetwork []string `json:"serviceNetwork,omitempty"`
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
}

func init() { register(&Cluster{}, &ClusterList{}) }
