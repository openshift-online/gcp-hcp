package maestroclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/manifest"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/transportclient"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/constants"
	apperrors "github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/errors"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/version"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/cert"
	grpcopts "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
	"sigs.k8s.io/yaml"
)

// Default configuration values
const (
	DefaultHTTPTimeout              = 10 * time.Second
	DefaultServerHealthinessTimeout = 20 * time.Second
)

// Client is the Maestro client for managing ManifestWorks via CloudEvents gRPC
type Client struct {
	workClient       workv1client.WorkV1Interface
	maestroAPIClient *openapi.APIClient
	config           *Config
	log              logger.Logger
	grpcOptions      *grpcopts.GRPCOptions
}

// Config holds configuration for creating a Maestro client
// Following the official Maestro client pattern:
// https://github.com/openshift-online/maestro/blob/main/examples/manifestwork/client.go
type Config struct {
	// MaestroServerAddr is the Maestro HTTP API server address
	// (e.g., "https://maestro.example.com:8000")
	// This is used for the OpenAPI client to communicate with Maestro's REST API
	MaestroServerAddr string

	// GRPCServerAddr is the Maestro gRPC server address
	// (e.g., "maestro-grpc.example.com:8090")
	// This is used for CloudEvents communication
	GRPCServerAddr string

	// SourceID is a unique identifier for this client
	// (used for CloudEvents routing)
	// This identifies the source of ManifestWork operations
	SourceID string

	// TLS Configuration for gRPC (optional - for secure connections)
	// CAFile is the path to the CA certificate file for verifying the gRPC server
	CAFile string
	// ClientCertFile is the path to the client certificate file for mutual TLS (gRPC)
	ClientCertFile string
	// ClientKeyFile is the path to the client key file for mutual TLS (gRPC)
	ClientKeyFile string
	// TokenFile is the path to a token file for token-based authentication
	// (alternative to cert auth)
	TokenFile string

	// TLS Configuration for HTTP API (optional - may use different CA than gRPC)
	// HTTPCAFile is the path to the CA certificate file for verifying the HTTPS API server
	// If not set, falls back to CAFile for backwards compatibility
	HTTPCAFile string

	// Insecure disables TLS verification and allows plaintext connections
	// Use this for local testing without TLS or with self-signed certificates
	// WARNING: NOT recommended for production
	Insecure bool

	// HTTPTimeout is the timeout for HTTP requests to Maestro API (default: 10s)
	HTTPTimeout time.Duration
	// ServerHealthinessTimeout is the timeout for gRPC server health checks
	// (default: 20s)
	ServerHealthinessTimeout time.Duration
}

// NewMaestroClient creates a new Maestro client using the official Maestro client pattern
//
// The client uses:
//   - Maestro HTTP API (OpenAPI client) for resource management
//   - CloudEvents over gRPC for ManifestWork operations
//
// Example Usage:
//
//	config := &Config{
//	    MaestroServerAddr: "https://maestro.example.com:8000",
//	    GRPCServerAddr:    "maestro-grpc.example.com:8090",
//	    SourceID:          "hyperfleet-adapter",
//	    CAFile:            "/etc/maestro/certs/ca.crt",
//	    ClientCertFile:    "/etc/maestro/certs/client.crt",
//	    ClientKeyFile:     "/etc/maestro/certs/client.key",
//	}
//	client, err := NewMaestroClient(ctx, config, log)
func NewMaestroClient(ctx context.Context, config *Config, log logger.Logger) (*Client, error) {
	if config == nil {
		return nil, apperrors.ConfigurationError("maestro config is required")
	}
	if config.MaestroServerAddr == "" {
		return nil, apperrors.ConfigurationError("maestro server address is required")
	}

	// Validate MaestroServerAddr URL scheme
	serverURL, err := url.Parse(config.MaestroServerAddr)
	if err != nil {
		return nil, apperrors.ConfigurationError("invalid MaestroServerAddr URL: %v", err)
	}
	// Require http or https scheme
	// (reject schemeless or other schemes like ftp://, grpc://, etc.)
	if serverURL.Scheme != "http" && serverURL.Scheme != "https" {
		return nil, apperrors.ConfigurationError(
			"MaestroServerAddr must use http:// or https:// scheme (got scheme %q in %q)",
			serverURL.Scheme, config.MaestroServerAddr)
	}
	// Enforce https when Insecure=false
	if !config.Insecure && serverURL.Scheme != "https" {
		return nil, apperrors.ConfigurationError(
			"MaestroServerAddr must use https:// scheme when Insecure=false (got %q); "+
				"use https:// URL or set Insecure=true for http:// connections",
			serverURL.Scheme)
	}

	if config.GRPCServerAddr == "" {
		return nil, apperrors.ConfigurationError("maestro gRPC server address is required")
	}
	if config.SourceID == "" {
		return nil, apperrors.ConfigurationError("maestro sourceID is required")
	}

	// Apply defaults
	httpTimeout := config.HTTPTimeout
	if httpTimeout == 0 {
		httpTimeout = DefaultHTTPTimeout
	}
	serverHealthinessTimeout := config.ServerHealthinessTimeout
	if serverHealthinessTimeout == 0 {
		serverHealthinessTimeout = DefaultServerHealthinessTimeout
	}

	log.WithFields(map[string]interface{}{
		"maestroServer": config.MaestroServerAddr,
		"grpcServer":    config.GRPCServerAddr,
		"sourceID":      config.SourceID,
	}).Info(ctx, "Creating Maestro client")

	// Create HTTP client with appropriate TLS configuration
	httpTransport, transportErr := createHTTPTransport(config)
	if transportErr != nil {
		return nil, apperrors.ConfigurationError("failed to create HTTP transport: %v", transportErr)
	}

	// Create Maestro HTTP API client (OpenAPI)
	maestroAPIClient := openapi.NewAPIClient(&openapi.Configuration{
		DefaultHeader: make(map[string]string),
		UserAgent:     version.UserAgent(),
		Debug:         false,
		Servers: openapi.ServerConfigurations{
			{
				URL:         config.MaestroServerAddr,
				Description: "Maestro API Server",
			},
		},
		OperationServers: map[string]openapi.ServerConfigurations{},
		HTTPClient: &http.Client{
			Transport: httpTransport,
			Timeout:   httpTimeout,
		},
	})

	// Create gRPC options
	grpcOptions := &grpcopts.GRPCOptions{
		Dialer:                   &grpcopts.GRPCDialer{},
		ServerHealthinessTimeout: &serverHealthinessTimeout,
	}
	grpcOptions.Dialer.URL = config.GRPCServerAddr

	// Configure TLS if certificates are provided
	if tlsErr := configureTLS(config, grpcOptions); tlsErr != nil {
		return nil, apperrors.ConfigurationError("failed to configure TLS: %v", tlsErr)
	}

	// Create the Maestro gRPC work client using the official pattern
	// This returns a workv1client.WorkV1Interface with Kubernetes-style API
	workClient, err := grpcsource.NewMaestroGRPCSourceWorkClient(
		ctx,
		newOCMLoggerAdapter(log),
		maestroAPIClient,
		grpcOptions,
		config.SourceID,
	)
	if err != nil {
		return nil, apperrors.MaestroError("failed to create Maestro work client: %v", err)
	}

	log.WithFields(map[string]interface{}{
		"sourceID": config.SourceID,
	}).Info(ctx, "Maestro client created successfully")

	return &Client{
		workClient:       workClient,
		maestroAPIClient: maestroAPIClient,
		config:           config,
		log:              log,
		grpcOptions:      grpcOptions,
	}, nil
}

// createHTTPTransport creates an HTTP transport with appropriate TLS configuration.
// It clones http.DefaultTransport to preserve important defaults like
// ProxyFromEnvironment, connection pooling, timeouts, etc., and only overrides TLS settings.
func createHTTPTransport(config *Config) (*http.Transport, error) {
	// Clone default transport to preserve ProxyFromEnvironment, DialContext,
	// MaxIdleConns, IdleConnTimeout, TLSHandshakeTimeout, etc.
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, apperrors.ConfigurationError("http.DefaultTransport is not *http.Transport").AsError()
	}
	transport := defaultTransport.Clone()

	// Build TLS config
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if config.Insecure {
		// Insecure mode: skip TLS verification
		// (works for both http:// and https://)
		tlsConfig.InsecureSkipVerify = true //nolint:gosec // Intentional
	} else {
		// Secure mode: load CA certificate if provided
		// HTTPCAFile takes precedence, falls back to CAFile for backwards compatibility
		httpCAFile := config.HTTPCAFile
		if httpCAFile == "" {
			httpCAFile = config.CAFile
		}

		if httpCAFile != "" {
			caCert, err := os.ReadFile(filepath.Clean(httpCAFile))
			if err != nil {
				return nil, err
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return nil, apperrors.ConfigurationError(
					"failed to parse CA certificate from %s", httpCAFile).AsError()
			}
			tlsConfig.RootCAs = caCertPool
		}
	}

	transport.TLSClientConfig = tlsConfig
	return transport, nil
}

// configureTLS sets up TLS configuration for the gRPC connection
func configureTLS(config *Config, grpcOptions *grpcopts.GRPCOptions) error {
	// Insecure mode: plaintext gRPC connection (no TLS)
	// Note: Unlike HTTP where InsecureSkipVerify allows both http:// and https://,
	// gRPC TLS always requires a TLS handshake on the server side.
	// For self-signed certs with gRPC, use CAFile instead of Insecure=true.
	if config.Insecure {
		grpcOptions.Dialer.TLSConfig = nil
		return nil
	}

	// Option 1: Mutual TLS with certificates
	if config.CAFile != "" && config.ClientCertFile != "" && config.ClientKeyFile != "" {
		certConfig := cert.CertConfig{
			CAFile:         config.CAFile,
			ClientCertFile: config.ClientCertFile,
			ClientKeyFile:  config.ClientKeyFile,
		}
		if err := certConfig.EmbedCerts(); err != nil {
			return err
		}

		tlsConfig, err := cert.AutoLoadTLSConfig(
			certConfig,
			func() (*cert.CertConfig, error) {
				c := cert.CertConfig{
					CAFile:         config.CAFile,
					ClientCertFile: config.ClientCertFile,
					ClientKeyFile:  config.ClientKeyFile,
				}
				if err := c.EmbedCerts(); err != nil {
					return nil, err
				}
				return &c, nil
			},
			grpcOptions.Dialer,
		)
		if err != nil {
			return err
		}
		grpcOptions.Dialer.TLSConfig = tlsConfig
		return nil
	}

	// Option 2: Token-based authentication with CA
	if config.CAFile != "" && config.TokenFile != "" {
		token, tokenErr := readTokenFile(config.TokenFile)
		if tokenErr != nil {
			return tokenErr
		}
		grpcOptions.Dialer.Token = token

		certConfig := cert.CertConfig{
			CAFile: config.CAFile,
		}
		if embedErr := certConfig.EmbedCerts(); embedErr != nil {
			return embedErr
		}

		tlsConfig, tlsErr := cert.AutoLoadTLSConfig(
			certConfig,
			func() (*cert.CertConfig, error) {
				c := cert.CertConfig{
					CAFile: config.CAFile,
				}
				if embedErr := c.EmbedCerts(); embedErr != nil {
					return nil, embedErr
				}
				return &c, nil
			},
			grpcOptions.Dialer,
		)
		if tlsErr != nil {
			return tlsErr
		}
		grpcOptions.Dialer.TLSConfig = tlsConfig
		return nil
	}

	// Option 3: CA only (server verification without client auth)
	if config.CAFile != "" {
		certConfig := cert.CertConfig{
			CAFile: config.CAFile,
		}
		if err := certConfig.EmbedCerts(); err != nil {
			return err
		}

		tlsConfig, err := cert.AutoLoadTLSConfig(
			certConfig,
			func() (*cert.CertConfig, error) {
				c := cert.CertConfig{
					CAFile: config.CAFile,
				}
				if err := c.EmbedCerts(); err != nil {
					return nil, err
				}
				return &c, nil
			},
			grpcOptions.Dialer,
		)
		if err != nil {
			return err
		}
		grpcOptions.Dialer.TLSConfig = tlsConfig
		return nil
	}

	// Fail fast: Insecure=false but no TLS configuration was provided
	// This prevents silently falling back to plaintext connections
	return fmt.Errorf(
		"no TLS configuration provided: set CAFile " +
			"(with optional ClientCertFile/ClientKeyFile or TokenFile) " +
			"or set Insecure=true for plaintext connections")
}

// readTokenFile reads a token from a file and trims whitespace.
// Returns an error if the file is empty or contains only whitespace.
func readTokenFile(path string) (string, error) {
	token, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(string(token))
	if trimmed == "" {
		return "", fmt.Errorf("token file %s is empty or contains only whitespace", path)
	}
	return trimmed, nil
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	if c.grpcOptions != nil && c.grpcOptions.Dialer != nil {
		return c.grpcOptions.Dialer.Close()
	}
	return nil
}

// WorkClient returns the underlying WorkV1Interface for ManifestWork operations
func (c *Client) WorkClient() workv1client.WorkV1Interface {
	return c.workClient
}

// SourceID returns the configured source ID
func (c *Client) SourceID() string {
	return c.config.SourceID
}

// TransportContext carries per-request routing information for the Maestro transport backend.
// Pass this as the TransportContext (any) in ApplyResource or method parameters.
type TransportContext struct {
	// ConsumerName is the target cluster name (Maestro consumer).
	// Required for all Maestro operations.
	ConsumerName string
}

// resolveTransportContext extracts the maestro TransportContext
// from the generic transport context.
// Returns nil if target is nil or wrong type.
func (c *Client) resolveTransportContext(
	target transportclient.TransportContext,
) *TransportContext {
	if target == nil {
		return nil
	}
	tc, ok := target.(*TransportContext)
	if !ok {
		return nil
	}
	return tc
}

// =============================================================================
// TransportClient Interface Implementation
// =============================================================================

// Ensure Client implements transportclient.TransportClient
var _ transportclient.TransportClient = (*Client)(nil)

// DeleteResource deletes a resource, implementing transportclient.TransportClient.
// For Maestro transport, the resource name is used as the ManifestWork name and the
// ConsumerName is resolved from the transport context.
// The propagationPolicy in opts is ignored — ManifestWork handles its own cleanup semantics.
func (c *Client) DeleteResource(
	ctx context.Context,
	_ schema.GroupVersionKind,
	_, name string,
	_ *transportclient.DeleteOptions,
	target transportclient.TransportContext,
) error {
	transportCtx := c.resolveTransportContext(target)
	if transportCtx == nil || transportCtx.ConsumerName == "" {
		return fmt.Errorf(
			"maestro TransportContext with ConsumerName is required for DeleteResource")
	}
	return c.DeleteManifestWork(ctx, transportCtx.ConsumerName, name)
}

// ApplyResource applies a rendered ManifestWork (JSON/YAML bytes) to the target cluster.
// It parses the bytes into a ManifestWork, then applies it via Maestro gRPC.
// Requires a *maestroclient.TransportContext with ConsumerName.
func (c *Client) ApplyResource(
	ctx context.Context,
	manifestBytes []byte,
	opts *transportclient.ApplyOptions,
	target transportclient.TransportContext,
) (*transportclient.ApplyResult, error) {
	if len(manifestBytes) == 0 {
		return nil, fmt.Errorf("manifest bytes cannot be empty")
	}

	// Resolve maestro transport context
	transportCtx := c.resolveTransportContext(target)
	if transportCtx == nil {
		return nil, fmt.Errorf(
			"maestro TransportContext is required: " +
				"pass *maestroclient.TransportContext as target")
	}

	consumerName := transportCtx.ConsumerName
	if consumerName == "" {
		return nil, fmt.Errorf(
			"consumer name (target cluster) is required: " +
				"set TransportContext.ConsumerName")
	}

	ctx = logger.WithMaestroConsumer(ctx, consumerName)

	// Parse bytes into ManifestWork
	work, err := parseManifestWork(manifestBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ManifestWork: %w", err)
	}

	// Set namespace to consumer name
	work.Namespace = consumerName

	// Apply the ManifestWork (create or update with generation comparison)
	result, err := c.ApplyManifestWork(ctx, consumerName, work)
	if err != nil {
		return nil, fmt.Errorf("failed to apply ManifestWork: %w", err)
	}

	return &transportclient.ApplyResult{
		Operation: result.Operation,
		Reason:    result.Reason,
	}, nil
}

// GetResource retrieves a resource by searching all ManifestWorks for the target consumer.
func (c *Client) GetResource(
	ctx context.Context,
	gvk schema.GroupVersionKind,
	namespace, name string,
	target transportclient.TransportContext,
) (*unstructured.Unstructured, error) {
	transportCtx := c.resolveTransportContext(target)
	consumerName := ""
	if transportCtx != nil {
		consumerName = transportCtx.ConsumerName
	}
	if consumerName == "" {
		gr := schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}
		return nil, apierrors.NewNotFound(gr, name)
	}

	ctx = logger.WithMaestroConsumer(ctx, consumerName)

	// If the GVK is ManifestWork, get the ManifestWork object directly
	if gvk.Kind == constants.ManifestWorkKind &&
		gvk.Group == constants.ManifestWorkGroup {
		work, err := c.GetManifestWork(ctx, consumerName, name)
		if err != nil {
			return nil, err
		}
		return workToUnstructured(work)
	}

	// Otherwise, list all ManifestWorks and search within their workloads
	workList, err := c.ListManifestWorks(ctx, consumerName, "")
	if err != nil {
		return nil, err
	}

	for i := range workList.Items {
		for _, m := range workList.Items[i].Spec.Workload.Manifests {
			obj, err := manifestToUnstructured(m)
			if err != nil {
				continue
			}

			if obj.GetKind() == gvk.Kind &&
				obj.GetAPIVersion() == gvk.GroupVersion().String() &&
				obj.GetNamespace() == namespace &&
				obj.GetName() == name {
				return obj, nil
			}
		}
	}

	gr := schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}
	return nil, apierrors.NewNotFound(gr, name)
}

// DiscoverResources discovers resources by searching all ManifestWorks for the target consumer.
// If the GVK is ManifestWork, it matches against the ManifestWork objects themselves.
// Otherwise, it searches within the workloads of each ManifestWork.
func (c *Client) DiscoverResources(
	ctx context.Context,
	gvk schema.GroupVersionKind,
	discovery manifest.Discovery,
	target transportclient.TransportContext,
) (*unstructured.UnstructuredList, error) {
	transportCtx := c.resolveTransportContext(target)
	consumerName := ""
	if transportCtx != nil {
		consumerName = transportCtx.ConsumerName
	}
	if consumerName == "" {
		return &unstructured.UnstructuredList{}, nil
	}

	ctx = logger.WithMaestroConsumer(ctx, consumerName)

	// List all ManifestWorks for this consumer
	workList, err := c.ListManifestWorks(ctx, consumerName, "")
	if err != nil {
		return nil, err
	}

	allItems := &unstructured.UnstructuredList{}

	// If discovering ManifestWork objects themselves,
	// match against the top-level objects
	if gvk.Kind == constants.ManifestWorkKind &&
		gvk.Group == constants.ManifestWorkGroup {
		for i := range workList.Items {
			workUnstructured, err := workToUnstructured(&workList.Items[i])
			if err != nil {
				continue
			}
			if manifest.MatchesDiscoveryCriteria(workUnstructured, discovery) {
				allItems.Items = append(allItems.Items, *workUnstructured)
			}
		}
		return allItems, nil
	}

	// Otherwise, search within each ManifestWork's workload
	for i := range workList.Items {
		workUnstructured, err := workToUnstructured(&workList.Items[i])
		if err != nil {
			continue
		}
		list, err := c.DiscoverManifestInWork(workUnstructured, discovery)
		if err != nil {
			continue
		}
		allItems.Items = append(allItems.Items, list.Items...)
	}

	return allItems, nil
}

// parseManifestWork parses JSON or YAML bytes into a ManifestWork object.
func parseManifestWork(data []byte) (*workv1.ManifestWork, error) {
	work := &workv1.ManifestWork{}

	// Try JSON first
	if err := json.Unmarshal(data, work); err == nil && work.Name != "" {
		return work, nil
	}

	// Fall back to YAML
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	if err := json.Unmarshal(jsonData, work); err != nil {
		return nil, fmt.Errorf("failed to parse ManifestWork: %w", err)
	}

	return work, nil
}

// determineOperation determines the operation that was performed based on the ManifestWork.

// manifestToUnstructured converts a workv1.Manifest to an unstructured object.
func manifestToUnstructured(m workv1.Manifest) (*unstructured.Unstructured, error) {
	if m.Raw == nil {
		return nil, fmt.Errorf("manifest has no raw data")
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(m.Raw, &obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	return &unstructured.Unstructured{Object: obj}, nil
}
