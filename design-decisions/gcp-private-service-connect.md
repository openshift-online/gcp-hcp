# GCP Private Service Connect Design Decisions

***Scope***: GCP-HCP

**Date**: 2025-10-29

## Overview

This document outlines the key architectural decisions for GCP Private Service Connect (PSC) integration in HyperShift. 

## Scope

This design covers the architectural foundation for Private Service Connect integration in HyperShift:

- **API Design**: Core GCP platform types and unified CRD for PSC lifecycle management
- **Network Architecture**: Clean separation between customer and management VPC configurations

## Key Design Decisions

### 1. Single CRD Architecture
- **Single `GCPPrivateServiceConnect` CRD** managing complete PSC lifecycle
- **Provides Unified status tracking** across Service Attachment, PSC Endpoint, and DNS phases

### 2. Separation of Network Configuration
- **Customer VPC Configuration** in `HostedCluster.Spec.Platform.GCP.CustomerNetworkConfig`
    - **Customer-focused fields** for PSC Endpoint creation
    - **Eliminates NodePool dependencies** to support zero-worker-node clusters, keeping kube API access
- **Poll of Subnets Auto-Provisioned:**: Hypershift Operator assumes there are pre-created subnets ready to be assined; Hypershift Operator assigns a free subnet during control plane creation

### 3. PSC Lifecycle Management
- **PSC Infrastructure Created Post-Cluster**: Private Service Connect infrastructure is provisioned after the HostedCluster creation as part of the control plane lifecycle
- **Zero-Worker-Node Support**: PSC connectivity is established even if the cluster doesn't have any NodePools, enabling control-plane-only deployments
- **Cluster Lifecycle Integration**: PSC resources are automatically deleted during cluster deprovisioning with proper cleanup ordering

## Example Configuration

The following example shows how the new API types are used in a HostedCluster specification:

```yaml
apiVersion: hypershift.openshift.io/v1beta1
kind: HostedCluster
metadata:
  name: <cluster-name>
  namespace: clusters
spec:
  platform:
    type: GCP
    gcp:
      # Management cluster configuration
      region: "<region>"
      project: "<management-project>"

      # Customer VPC configuration for PSC
      customerNetworkConfig:
        # Customer project for PSC endpoint
        project: "<customer-project>"
        # Customer VPC network
        network:
          name: "<customer-network>"
        # Customer subnet for PSC endpoint and workers
        pscSubnet:
          name: "<customer-psc-subnet>"

      # Platform-level endpoint access control
      endpointAccess: "Private"

```

This configuration demonstrates the clean separation between management infrastructure (auto-provisioned) and customer network configuration (user-specified).

## Implementation Details

For detailed implementation guidance including API type definitions, controller integration patterns, validation rules, and testing strategies, see the [GCP Private Service Connect Implementation Plan](../implementation-plans/gcp-private-service-connect-implementation.md).