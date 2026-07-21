# Orlop API Server Architecture

## Dual API Surface with Shared Storage

```
┌─────────────────────────────────────────────────────────────────────┐
│                          API Clients                                │
└──────────────┬─────────────────────────────┬────────────────────────┘
               │                             │
               │                             │
       ┌───────▼────────┐            ┌───────▼────────┐
       │  Private API   │            │   Public API   │
       │   Port 8080    │            │   Port 8081    │
       └───────┬────────┘            └───────┬────────┘
               │                             │
               │                             │
       ┌───────▼────────┐            ┌───────▼────────┐
       │ Private Schema │            │ Public Schema  │
       │  (Internal)    │            │  (Filtered)    │
       └───────┬────────┘            └───────┬────────┘
               │                             │
               │                    ┌────────▼────────┐
               │                    │   Converter     │
               │                    │ PrivateToPublic │
               │                    │ PublicToPrivate │
               │                    └────────┬────────┘
               │                             │
               └──────────┬──────────────────┘
                          │
                  ┌───────▼───────┐
                  │ Shared Store  │
                  │  (In-Memory)  │
                  │               │
                  │  Objects are  │
                  │ stored in the │
                  │ PRIVATE type  │
                  └───────────────┘
```

## API Flow Examples

### Creating an Object via Private API

```
Client → POST /apis/.../v1/namespaces/default/objects
         {
           spec: {publicField: "val", internalField: "secret"},
           metadata: {
             labels: {"app": "web", "private.orlop.../key": "value"}
           }
         }
         │
         ▼
  Private Schema Validation
         │
         ▼
  Store (Private Type) ✓ All fields stored
```

### Reading via Public API

```
Client → GET /apis/.../v1/namespaces/default/objects/test
         │
         ▼
  Store (Private Type) → {spec: {publicField, internalField}, 
                          metadata: {labels: {app, private.orlop...}}}
         │
         ▼
  Converter.PrivateToPublic()
    1. JSON round-trip (drops internalField)
    2. filterPrivateMetadata() (removes private.orlop... labels/annotations)
    3. filterPrivateConditions() (removes private.orlop... conditions)
         │
         ▼
  Client ← {spec: {publicField: "val"},
            metadata: {labels: {"app": "web"}}}
            ✓ Internal fields hidden
```

### Updating via Public API

```
Client → PUT /apis/.../v1/namespaces/default/objects/test
         {
           spec: {publicField: "newval"},
           metadata: {resourceVersion: "5"}
         }
         │
         ▼
  Converter.PublicToPrivate(public, existing)
    1. Start with existing private object (preserves internalField)
    2. Overlay public fields from request
    3. Result: {spec: {publicField: "newval", internalField: "secret"}}
         │
         ▼
  Store (Private Type) ✓ Internal fields preserved
```

## Key Components

### Shared Storage
- Single in-memory store per resource type
- Objects stored in **private type** format
- Both APIs access the same store instances
- Created in `privateRegistry`, referenced by `publicRegistry`

### Converter
**Location:** `pkg/apiserver/conversion/conversion.go`

**Methods:**
- `PrivateToPublic(private) → public`
  - JSON round-trip automatically filters private-only fields
  - `filterPrivateMetadata()` removes labels/annotations with `private.orlop.thetechnick.ninja/` prefix
  - `filterPrivateConditions()` removes conditions with `private.orlop.thetechnick.ninja/` prefix

- `PublicToPrivate(public, existing) → private`
  - Starts with existing object to preserve internal fields
  - Overlays public data on top
  - Used for CREATE and UPDATE operations

### Schema Types

**Private Schema:**
```go
// All fields visible
type ObjectSpec struct {
    PublicField   string `json:"publicField"`
    InternalField string `json:"internalField"`  // Not tagged +orlop:public
}
```

**Public Schema:**
```go
// Only public fields
type ObjectSpec struct {
    PublicField string `json:"publicField"`
    // internalField omitted
}
```

### Router Setup

**Private Router:**
```go
privateRegistry := NewResourceRegistry(privateScheme)
privateRegistry.Register(privateResources...)
router := setupRouter(privateRegistry)  // Direct handlers
```

**Public Router:**
```go
publicRegistry := NewResourceRegistry(publicScheme)
publicRegistry.Register(publicResources...)

// Create converting handlers that:
//   - Use privateRegistry.GetStore() (shared storage)
//   - Use publicRegistry schemas (filtering)
//   - Use converter for type conversion
router := setupConvertingRouter(publicRegistry, privateRegistry, converter)
```

## Security Model

### Private API (Port 8080)
- Full access to all fields
- No filtering applied
- Intended for cluster-internal use
- Exposes `internalField`, private labels, private annotations, private conditions

### Public API (Port 8081)
- Filtered view of the same data
- Private fields automatically hidden via schema
- Private metadata explicitly filtered via converter
- Intended for external consumers
- Never exposes fields/metadata prefixed with `private.orlop.thetechnick.ninja/`

## Benefits

1. **Single Source of Truth:** One storage backend prevents data inconsistency
2. **Automatic Filtering:** Schema + converter ensure private data stays private
3. **Preserved Semantics:** Public API updates preserve internal state
4. **Standard Kubernetes Patterns:** Uses standard GVK, resourceVersion, etc.
5. **Type Safety:** Separate schemes prevent accidental exposure
