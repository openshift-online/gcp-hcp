package constants

// HTTP Headers
const (
	// HeaderContentType is the Content-Type HTTP header.
	HeaderContentType = "Content-Type"

	// HeaderAccept is the Accept HTTP header for content negotiation.
	HeaderAccept = "Accept"

	// HeaderVary is the Vary HTTP header for cache control.
	HeaderVary = "Vary"

	// HeaderLastModified is the Last-Modified HTTP header.
	HeaderLastModified = "Last-Modified"

	// HeaderTransferEncoding is the Transfer-Encoding HTTP header.
	HeaderTransferEncoding = "Transfer-Encoding"
)

// Content Types
const (
	// ContentTypeJSON is the standard JSON content type.
	ContentTypeJSON = "application/json"

	// ContentTypeJSONPatch is the JSON Patch content type (RFC 6902).
	ContentTypeJSONPatch = "application/json-patch+json"

	// ContentTypeMergePatch is the JSON Merge Patch content type (RFC 7386).
	ContentTypeMergePatch = "application/merge-patch+json"

	// ContentTypeStrategicMergePatch is the Kubernetes Strategic Merge Patch content type.
	ContentTypeStrategicMergePatch = "application/strategic-merge-patch+json"

	// ContentTypeApplyPatchPrefix is the prefix for server-side apply patch content types.
	ContentTypeApplyPatchPrefix = "application/apply-patch+"

	// ContentTypeOpenAPIV2Protobuf is the OpenAPI v2 protobuf content type.
	ContentTypeOpenAPIV2Protobuf = "application/com.github.proto-openapi.spec.v2.v1.0+protobuf"

	// ContentTypeOpenAPIV2ProtobufAlt is an alternative OpenAPI v2 protobuf content type.
	ContentTypeOpenAPIV2ProtobufAlt = "application/com.github.proto-openapi.spec.v2@v1.0+protobuf"

	// TransferEncodingChunked is the chunked transfer encoding value.
	TransferEncodingChunked = "chunked"
)

// JSON Field Names - Core Kubernetes Object Fields
const (
	// FieldMetadata is the metadata field in Kubernetes objects.
	FieldMetadata = "metadata"

	// FieldSpec is the spec field in Kubernetes objects.
	FieldSpec = "spec"

	// FieldStatus is the status field in Kubernetes objects.
	FieldStatus = "status"

	// FieldResourceVersion is the resourceVersion field in metadata.
	FieldResourceVersion = "resourceVersion"

	// FieldAPIVersion is the apiVersion field in Kubernetes objects.
	FieldAPIVersion = "apiVersion"

	// FieldKind is the kind field in Kubernetes objects.
	FieldKind = "kind"

	// FieldItems is the items array field in list responses.
	FieldItems = "items"

	// FieldLabels is the labels field in metadata.
	FieldLabels = "labels"

	// FieldAnnotations is the annotations field in metadata.
	FieldAnnotations = "annotations"

	// FieldName is the name field in metadata.
	FieldName = "name"

	// FieldNamespace is the namespace field in metadata.
	FieldNamespace = "namespace"
)

// URL Parameters
const (
	// URLParamNamespace is the namespace URL parameter name.
	URLParamNamespace = "namespace"

	// URLParamName is the name URL parameter name.
	URLParamName = "name"
)

// Query Parameters
const (
	// QueryParamLabelSelector is the labelSelector query parameter.
	QueryParamLabelSelector = "labelSelector"

	// QueryParamWatch enables watch mode.
	QueryParamWatch = "watch"

	// QueryParamResourceVersion specifies the resource version for watch.
	QueryParamResourceVersion = "resourceVersion"

	// QueryParamAllowWatchBookmarks enables bookmark events in watch.
	QueryParamAllowWatchBookmarks = "allowWatchBookmarks"

	// QueryParamSendInitialEvents sends existing objects as ADDED events in watch.
	QueryParamSendInitialEvents = "sendInitialEvents"

	// QueryParamResourceVersionMatch specifies resource version matching strategy.
	QueryParamResourceVersionMatch = "resourceVersionMatch"

	// QueryParamTimeoutSeconds sets the watch timeout.
	QueryParamTimeoutSeconds = "timeoutSeconds"

	// QueryParamLimit limits the number of items returned in list.
	QueryParamLimit = "limit"

	// QueryParamContinue is the continuation token for pagination.
	QueryParamContinue = "continue"

	// QueryParamFieldManager identifies the field manager for server-side apply.
	QueryParamFieldManager = "fieldManager"

	// QueryParamForce forces server-side apply conflicts.
	QueryParamForce = "force"

	// QueryParamPropagationPolicy specifies deletion propagation policy.
	QueryParamPropagationPolicy = "propagationPolicy"

	// QueryParamShardIndex specifies the shard index for distributed queries.
	QueryParamShardIndex = "shardIndex"

	// QueryParamShardCount specifies the total number of shards.
	QueryParamShardCount = "shardCount"
)

// Kubernetes Kinds
const (
	// KindStatus is the Status kind for error responses.
	KindStatus = "Status"

	// KindAPIGroup is the APIGroup kind for discovery.
	KindAPIGroup = "APIGroup"

	// KindAPIGroupList is the APIGroupList kind for discovery.
	KindAPIGroupList = "APIGroupList"

	// KindAPIResourceList is the APIResourceList kind for discovery.
	KindAPIResourceList = "APIResourceList"
)

// API Versions
const (
	// APIVersionV1 is the Kubernetes core API version.
	APIVersionV1 = "v1"
)

// Deletion Propagation Policies
const (
	// PropagationPolicyOrphan removes owner references but keeps dependents.
	PropagationPolicyOrphan = "Orphan"

	// PropagationPolicyBackground deletes the object and garbage collects dependents asynchronously.
	PropagationPolicyBackground = "Background"

	// PropagationPolicyForeground deletes dependents before deleting the owner.
	PropagationPolicyForeground = "Foreground"
)

// Kubernetes Annotations
const (
	// AnnotationInitialEventsEnd marks the end of initial events in watch.
	AnnotationInitialEventsEnd = "k8s.io/initial-events-end"
)
