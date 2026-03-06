# GCP Private Service Connect Implementation Plan

***Scope***: GCP-HCP

**Date**:

2026-03-02 - update plan to match implementation
2025-11-03 - update controller architecture
2025-10-30 (original)

## Overview

This document provides detailed implementation guidance for GCP Private Service Connect (PSC) API types and CRD infrastructure in HyperShift. This document implements the architectural decisions outlined in the [GCP Private Service Connect Design Document](../design-decisions/gcp-private-service-connect-implementation.md).

## Implementation Scope

This implementation covers two main deliverables:

**Card 1: GCP PSC API Types and CRD Infrastructure**
- GCP platform types (GCPResourceReference, GCPEndpointAccessType, GCPNetworkConfig)
- Extended GCPPlatformSpec with NetworkConfig
- GCPPrivateServiceConnect CRD definition and creation
- Feature gates, validation, scheme registration, and manifest generation

**Card 2: Controller Architecture**
- Dedicated controller in hypershift-operator for Service Attachment creation
- Dedicated controllers in control-plane-operator for PSC Endpoint and DNS creation

## Card 1: GCP PSC API Types and CRD Infrastructure

**Story**: As a HyperShift developer, I want to create comprehensive GCP platform types and CRD infrastructure for Private Service Connect functionality, so that customers can configure PSC networking and the control plane can manage PSC connectivity as a unified Kubernetes resource.

### Minimal API Strategy
Define the minimal required API types for PSC functionality to implement PSC with the smallest possible API surface and follow established patterns. This implements the minimal viable API for PSC functionality.

### Core GCP Platform Types

**GCPResourceReference**
```go
// GCPResourceReference represents a reference to a GCP resource by name
// Follows GCP naming patterns (name-based APIs, not ID-based like AWS)
// See https://google.aip.dev/122 for GCP resource name standards
type GCPResourceReference struct {
    // Name is the name of the GCP resource
    Name string `json:"name"`
}
```

**GCPEndpointAccessType**
```go
// GCPEndpointAccessType defines the endpoint access type for GCP clusters
// Equivalent to AWS EndpointAccessType but adapted for GCP networking model
type GCPEndpointAccessType string

const (
    // PublicAndPrivate endpoint access allows public API server access and
    // private node communication with the control plane via PSC.
    GCPEndpointAccessPublicAndPrivate GCPEndpointAccessType = "PublicAndPrivate"

    // Private endpoint access allows only private API server access and private
    // node communication with the control plane via PSC.
    GCPEndpointAccessPrivate GCPEndpointAccessType = "Private"
)
```

**GCPNetworkConfig**
```go
// GCPNetworkConfig specifies VPC configuration for GCP clusters and Private Service Connect endpoint creation.
type GCPNetworkConfig struct {
    // Network is the VPC network name
    // +required
    // +immutable
    Network GCPResourceReference `json:"network"`

    // PrivateServiceConnectSubnet is the subnet for Private Service Connect endpoints
    // +required
    // +immutable
    PrivateServiceConnectSubnet GCPResourceReference `json:"privateServiceConnectSubnet"`
}
```


### Extended GCPPlatformSpec
```go
// Extend existing GCPPlatformSpec with networking configuration and endpoint access
type GCPPlatformSpec struct {
    // ... existing fields ...

    // NetworkConfig specifies VPC configuration for Private Service Connect.
    // Required for VPC configuration in Private Service Connect deployments.
    // +required
    NetworkConfig GCPNetworkConfig `json:"networkConfig"`

    // EndpointAccess controls API endpoint accessibility for the HostedControlPlane on GCP.
    // Allowed values: "Private", "PublicAndPrivate". Defaults to "Private".
    // +kubebuilder:validation:Enum=PublicAndPrivate;Private
    // +kubebuilder:default=Private
    // +optional
    EndpointAccess GCPEndpointAccessType `json:"endpointAccess,omitempty"`
}
```

### GCPPrivateServiceConnect CRD

**Spec Structure**
```go
// GCPPrivateServiceConnectSpec defines the desired state of PSC infrastructure
type GCPPrivateServiceConnectSpec struct {
    // LoadBalancerIP is the IP address of the Internal Load Balancer
    LoadBalancerIP string `json:"loadBalancerIP"`

    // ForwardingRuleName is the name of the Internal Load Balancer forwarding rule
    ForwardingRuleName string `json:"forwardingRuleName,omitempty"`

    // ConsumerAcceptList specifies which customer projects can connect
    ConsumerAcceptList []string `json:"consumerAcceptList"`

    // NATSubnet is the subnet used for NAT by the Service Attachment
    NATSubnet string `json:"natSubnet,omitempty"`
}
```

**Status Structure**
```go
// GCPPrivateServiceConnectStatus defines the observed state of PSC infrastructure
type GCPPrivateServiceConnectStatus struct {
    // Conditions represent the current state of PSC infrastructure
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // Management Side Status (Service Attachment)

    // ServiceAttachmentName is the name of the created Service Attachment
    ServiceAttachmentName string `json:"serviceAttachmentName,omitempty"`

    // ServiceAttachmentURI is the URI customers use to connect
    // Format: projects/{project}/regions/{region}/serviceAttachments/{name}
    ServiceAttachmentURI string `json:"serviceAttachmentURI,omitempty"`

    // Customer Side Status (PSC Endpoint and DNS)

    // EndpointIP is the reserved IP address for the PSC endpoint
    EndpointIP string `json:"endpointIP,omitempty"`

    // DNSZones contains DNS zone information created for this cluster
    DNSZones []DNSZoneStatus `json:"dnsZones,omitempty"`
}

**DNSZoneStatus**
```go
// DNSZoneStatus represents DNS zone information
type DNSZoneStatus struct {
    // Name is the DNS zone name
    Name string `json:"name"`

    // Records lists the DNS records created in this zone
    Records []string `json:"records,omitempty"`
}
```
```

**Complete CRD Definition**
```go
// GCPPrivateServiceConnect represents GCP Private Service Connect infrastructure
type GCPPrivateServiceConnect struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   GCPPrivateServiceConnectSpec   `json:"spec,omitempty"`
    Status GCPPrivateServiceConnectStatus `json:"status,omitempty"`
}
```

### CRD Infrastructure Implementation

**File Location**: `/api/hypershift/v1beta1/gcpprivateserviceconnect_types.go`

Complete CRD definition following AWSEndpointService pattern with:
- **GCPPrivateServiceConnectSpec**: LoadBalancerIP, ForwardingRuleName, ConsumerAcceptList, NATSubnet
- **GCPPrivateServiceConnectStatus**: Comprehensive status tracking for both sides
  - Management side: ServiceAttachmentName, ServiceAttachmentURI (like AWS EndpointServiceName)
  - Customer side: EndpointIP, DNSZones (like AWS EndpointID, DNSNames, DNSZoneID)

### Feature Gate Integration

```go
type GCPPrivateServiceConnect struct {
    // ... CRD definition
    // Feature gate: GCPPlatform
}

type GCPPrivateServiceConnectList struct {
    // ... List type definition
    // Feature gate: GCPPlatform
}
```

**Requirements**:
- CRD properly gated behind GCPPlatform feature flag for controlled rollout
- Featuregated manifests generated in correct directories with GCPPlatform.yaml naming
- Only available when explicitly enabled

### Scheme Registration and Client Generation

```go
func init() {
    SchemeBuilder.Register(&GCPPrivateServiceConnect{}, &GCPPrivateServiceConnectList{})
}
```


### CRD Manifest Generation

**Generated Files**:
```
/cmd/install/assets/hypershift-operator/zz_generated.crd-manifests/
├── gcpprivateserviceconnects-CustomNoUpgrade.crd.yaml
└── gcpprivateserviceconnects-TechPreviewNoUpgrade.crd.yaml

/api/hypershift/v1beta1/zz_generated.featuregated-crd-manifests/
└── gcpprivateserviceconnects.hypershift.openshift.io/
    └── GCPPlatform.yaml
```

**Requirements**:
- CRD YAML includes all validation rules
- Proper OpenAPI schema with validation constraints
- Featuregated CRD manifests in appropriate directories

### Comprehensive Validation

**Field Validation Requirements**:
- Required fields, string patterns, array constraints, length limits
- Clear error messages for field-level validation
- GCP-specific validation (project ID format, resource naming patterns)
- Namespace scoping and RBAC considerations

## Card 2: Controller Architecture

Add a dedicated controller to hypershift-operator for Service Attachment creation

Add dedicated controllers to control-plane-operator for PSC Endpoint and DNS creation

### Controller Architecture Overview


```
Management Side (hypershift-operator):
- New GCPPrivateServiceConnectReconciler Controller
  - Watches GCPPrivateServiceConnect CRDs
  - Handles Service Attachment creation in management VPC

Customer Side (control-plane-operator):
- New GCPPrivateServiceObserver Controller
  - Watches router service with ILB
  - Creates GCPPrivateServiceConnect CRD when ILB is ready
  - Extracts ForwardingRuleName from LoadBalancer status

- New GCPPrivateServiceConnectReconciler Controller
  - Watches GCPPrivateServiceConnect CRD
  - Handles PSC Endpoint + DNS creation in customer VPC
```

### Management Side

**Files to Create**:
- `/hypershift-operator/controllers/platform/gcp/privateserviceconnect_controller.go` (new management-side controller)
- `/hypershift-operator/main.go` (add controller setup)

**Management Controller Responsibilities**:
- Watch GCPPrivateServiceConnect CRDs
- Create Service Attachment in management VPC
- Update status.ServiceAttachmentURI
- Add NATSubnet to spec


### Customer Side

**Files to Create**:
- `/control-plane-operator/controllers/gcpprivateserviceconnect/observer.go` (GCPPrivateServiceObserver controller)
- `/control-plane-operator/controllers/gcpprivateserviceconnect/psc_endpoint_controller.go` (GCPPrivateServiceConnectReconciler controller)
- `/control-plane-operator/controllers/gcpprivateserviceconnect/dns.go` (DNS zone and record management)
- `/control-plane-operator/main.go` (add controller setup)

**GCPPrivateServiceObserver Controller Responsibilities**:
- Watch router service with ILB
- Create GCPPrivateServiceConnect CRD when ILB is ready
- Extract ForwardingRuleName from LoadBalancer status

**GCPPrivateServiceConnectReconciler Controller Responsibilities**:
- Watch GCPPrivateServiceConnect CRDs in control plane namespace
- Create PSC Endpoint in customer VPC
- Create DNS zone and records in customer project
- Update status.EndpointIP, status.DNSZones

### Status Propagation and Monitoring

**Status Conditions**:
- `GCPServiceAttachmentAvailable`: Management side Service Attachment status
- `GCPEndpointAvailable`: Customer side PSC Endpoint status
- `GCPDNSAvailable`: DNS configuration status
- `GCPPrivateServiceConnectAvailable`: Overall PSC infrastructure readiness (computed)

## Validation and Testing Strategy

### API Validation
- **Field Validation**: Required fields, string patterns, array constraints
- **Cross-Field Validation**: Logical consistency between network configuration fields
- **GCP-Specific Validation**: Project ID format, resource naming patterns
- **Immutability**: Critical fields marked immutable where appropriate

### Unit Testing
- **API Type Validation**: Valid and invalid resource examples
- **CRD Generation**: Verify generated manifests include all validation rules
- **Feature Gates**: Test CRD availability with GCPPlatform feature flag
- **Status Propagation**: Verify PSC status reflects in HostedCluster conditions

## Generated Artifacts
- API types in `/api/hypershift/v1beta1/`
- CRD YAML manifests in `/cmd/install/assets/hypershift-operator/`
- Featuregated manifests in `/api/hypershift/v1beta1/zz_generated.featuregated-crd-manifests/`
- Deepcopy methods and client code generation