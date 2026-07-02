package hyperfleetapi

// ClusterDetail is returned by GET /clusters/{id}.
type ClusterDetail struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Generation int64         `json:"generation"`
	CreatedBy  string        `json:"created_by"`
	Spec       ClusterSpec   `json:"spec"`
	Status     ClusterStatus `json:"status"`
}

// ClusterSpec holds the desired-state specification for a cluster.
type ClusterSpec struct {
	InfraID   string      `json:"infraID"`
	IssuerURL string      `json:"issuerURL,omitempty"`
	ClusterID string      `json:"clusterID,omitempty"` // RFC4122 UUID for HC spec.clusterID
	Release   ReleaseSpec `json:"release,omitempty"`
	Platform  GCPPlatform `json:"platform"`
}

// ReleaseSpec identifies an OCP release version.
type ReleaseSpec struct {
	Version string `json:"version"`
}

// GCPPlatform is the top-level platform envelope: {"type":"GCP","gcp":{...}}.
type GCPPlatform struct {
	Type string    `json:"type"`
	GCP  GCPConfig `json:"gcp"`
}

// GCPConfig holds the GCP-specific cluster platform fields nested under "gcp".
type GCPConfig struct {
	ProjectID        string    `json:"projectID"`
	Region           string    `json:"region"`
	Network          string    `json:"network"`
	Subnet           string    `json:"subnet"`
	EndpointAccess   string    `json:"endpointAccess,omitempty"`
	WorkloadIdentity WIFConfig `json:"workloadIdentity"`
}

// WIFConfig holds Workload Identity Federation configuration.
type WIFConfig struct {
	ProjectNumber      string             `json:"projectNumber"`
	PoolID             string             `json:"poolID"`
	ProviderID         string             `json:"providerID"`
	ServiceAccountsRef WIFServiceAccounts `json:"serviceAccountsRef"`
}

// WIFServiceAccounts holds the GCP service account e-mails for each component.
type WIFServiceAccounts struct {
	NodePool        string `json:"nodePoolEmail"`
	ControlPlane    string `json:"controlPlaneEmail"`
	CloudController string `json:"cloudControllerEmail"`
	Storage         string `json:"storageEmail"`
	ImageRegistry   string `json:"imageRegistryEmail"`
	Network         string `json:"networkEmail"`
}

// ClusterStatus holds observed state for a cluster.
type ClusterStatus struct {
	Conditions []Condition `json:"conditions,omitempty"`
}

// NodePoolDetail is returned by GET /clusters/{id}/nodepools/{id}.
type NodePoolDetail struct {
	ID         string         `json:"id"`
	ClusterID  string         `json:"clusterId"`
	Name       string         `json:"name"`
	Generation int64          `json:"generation"`
	Spec       NodePoolSpec   `json:"spec"`
	Status     NodePoolStatus `json:"status"`
}

// NodePoolSpec holds the desired-state specification for a node pool.
type NodePoolSpec struct {
	Release  ReleaseSpec         `json:"release,omitempty"`
	Platform NodePoolGCPPlatform `json:"platform"`
}

// NodePoolGCPPlatform is the top-level nodepool platform envelope: {"type":"GCP","gcp":{...}}.
type NodePoolGCPPlatform struct {
	Type string          `json:"type"`
	GCP  NodePoolGCPConf `json:"gcp"`
}

// NodePoolGCPConf holds the GCP-specific node pool platform fields nested under "gcp".
type NodePoolGCPConf struct {
	ProjectID string `json:"projectID"`
	Region    string `json:"region"`
	Zone      string `json:"zone,omitempty"` // optional; falls back to region+"-a" in templates
}

// NodePoolStatus holds observed state for a node pool.
type NodePoolStatus struct {
	Conditions []Condition `json:"conditions,omitempty"`
}

// Condition is a standard Kubernetes-style status condition.
type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// AdapterStatus is one adapter's status entry returned by the statuses endpoints.
type AdapterStatus struct {
	Adapter    string         `json:"adapter"`
	Conditions []Condition    `json:"conditions,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
}

// AdapterStatuses is a slice of AdapterStatus with typed accessor helpers.
type AdapterStatuses []AdapterStatus

// find returns the first entry whose Adapter field equals name, or nil.
func (ss AdapterStatuses) find(name string) *AdapterStatus {
	for i := range ss {
		if ss[i].Adapter == name {
			return &ss[i]
		}
	}
	return nil
}

// stringField extracts a string field from a map[string]any, returning "" if missing or wrong type.
func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// boolField extracts a bool field from a map[string]any.
func boolField(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// PlacementData holds the output from the placement-adapter.
type PlacementData struct {
	ManagementClusterName string
	BaseDomain            string
}

// Ready returns true when both ManagementClusterName and BaseDomain are non-empty.
func (p *PlacementData) Ready() bool {
	return p != nil && p.ManagementClusterName != "" && p.BaseDomain != ""
}

// Placement returns placement-adapter data, or nil if not present.
func (ss AdapterStatuses) Placement() *PlacementData {
	s := ss.find("placement-adapter")
	if s == nil {
		return nil
	}
	return &PlacementData{
		ManagementClusterName: stringField(s.Data, "managementClusterName"),
		BaseDomain:            stringField(s.Data, "baseDomain"),
	}
}

// VRData holds the output from the version-resolution-adapter (cluster level).
type VRData struct {
	ReleaseImage        string
	ReleaseVersion      string
	ReleaseChannel      string
	ReleaseChannelGroup string
}

// Ready returns true when ReleaseImage is non-empty.
func (v *VRData) Ready() bool {
	return v != nil && v.ReleaseImage != ""
}

// VersionResolution returns version-resolution-adapter data, or nil if not present.
func (ss AdapterStatuses) VersionResolution() *VRData {
	s := ss.find("version-resolution-adapter")
	if s == nil {
		return nil
	}
	return &VRData{
		ReleaseImage:        stringField(s.Data, "release_image"),
		ReleaseVersion:      stringField(s.Data, "release_version"),
		ReleaseChannel:      stringField(s.Data, "release_channel"),
		ReleaseChannelGroup: stringField(s.Data, "release_channel_group"),
	}
}

// NodePoolVRData holds the output from the nodepool-vr-adapter.
type NodePoolVRData struct {
	ReleaseImage        string
	ReleaseVersion      string
	ReleaseChannel      string
	ReleaseChannelGroup string
}

// Ready returns true when ReleaseImage is non-empty.
func (v *NodePoolVRData) Ready() bool {
	return v != nil && v.ReleaseImage != ""
}

// NodePoolVR returns nodepool-vr-adapter data, or nil if not present.
func (ss AdapterStatuses) NodePoolVR() *NodePoolVRData {
	s := ss.find("nodepool-vr-adapter")
	if s == nil {
		return nil
	}
	return &NodePoolVRData{
		ReleaseImage:        stringField(s.Data, "release_image"),
		ReleaseVersion:      stringField(s.Data, "release_version"),
		ReleaseChannel:      stringField(s.Data, "release_channel"),
		ReleaseChannelGroup: stringField(s.Data, "release_channel_group"),
	}
}

// HCData holds the output from the hc-adapter.
type HCData struct {
	IsAvailable bool
}

// Available returns true when the hc-adapter entry exists and IsAvailable is true.
func (h *HCData) Available() bool {
	return h != nil && h.IsAvailable
}

// HCAdapter returns hc-adapter data, or nil if not present.
// IsAvailable is derived from the hc-adapter's Available condition being "True".
func (ss AdapterStatuses) HCAdapter() *HCData {
	s := ss.find("hc-adapter")
	if s == nil {
		return nil
	}
	available := false
	for _, c := range s.Conditions {
		if c.Type == "Available" && c.Status == "True" {
			available = true
			break
		}
	}
	// Also check the data field for an "available" bool if present.
	if !available {
		available = boolField(s.Data, "available")
	}
	return &HCData{IsAvailable: available}
}

// StatusPayload is the request body for PUT /clusters/{id}/statuses and
// PUT /clusters/{id}/nodepools/{id}/statuses.
type StatusPayload struct {
	Adapter            string         `json:"adapter"`
	Conditions         []Condition    `json:"conditions,omitempty"`
	ObservedGeneration int64          `json:"observed_generation"`
	ObservedTime       string         `json:"observed_time"`
	Data               map[string]any `json:"data,omitempty"`
}
