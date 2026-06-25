# kube-applier: GCP HCP vs ARO HCP Alignment Assessment

This document compares the GCP HCP kube-applier implementation against the
ARO HCP upstream to identify divergences, gaps, and alignment opportunities
before proposing changes to either side.

**GCP HCP source**: `experiments/kube-applier-gcp/readme.md`
**ARO HCP source**: `kube-applier/readme.md` ([main branch](https://github.com/Azure/ARO-HCP/blob/main/kube-applier/readme.md))

---

## Shared Design

Both implementations share the same core architecture and controller logic:

- Three desire types: `ApplyDesire`, `DeleteDesire`, `ReadDesire`
- Single management cluster scope per binary instance
- Scale assumption: ~10k desires per MC, trivially manageable
- One-resource-per-desire constraint (no list/label selection)
- `Successful` and `Degraded` status conditions with identical semantics
- SSA with `force=true`, same adoption trade-offs
- Per-instance `ReadDesireKubernetesController` managed by a lifecycle controller
- `DeleteDesire` polling with finalizer-aware "WaitingForDeletion" state
- envtest-based integration tests with real kube-apiserver + etcd

---

## Infrastructure Divergences

These are intentional differences driven by the underlying cloud platform.

| Area | ARO HCP (CosmosDB) | GCP HCP (Firestore) |
|------|---------------------|---------------------|
| Database engine | Azure Cosmos DB (NoSQL) | Google Cloud Firestore (Native mode, regional) |
| Topology | Single Cosmos container per MC | Two Firestore named databases per MC (`specs` + `status`) |
| Spec/status co-location | Same container, same document | Separate databases with IAM-enforced directional isolation |
| Document IDs | ARM resource IDs (`subscriptions/{sub}/resourceGroups/...`) | Deterministic UUID v5 from `{taskKey}/{group}/{version}/{resource}/{namespace}/{name}` |
| Change notification | Cosmos change feed | Firestore real-time snapshot listeners (persistent gRPC streams) |
| Authentication | Cosmos credentials scoped per container | GKE Workload Identity Federation, no static credentials |
| Partition / isolation | Per-container credentials; partition key = MC name | Per-database IAM conditions at the Firestore database level |

### Key architectural difference: dual-database model

GCP HCP uses two Firestore databases per MC (`specs` and `status`) instead of
a single container. This enforces directional isolation at the IAM level: the
agent cannot write specs and the backend cannot write status. The trade-off is
that the status document may not exist when a spec is first reconciled, so the
`desirestatuswriter` uses a create-or-replace pattern.

ARO HCP co-locates spec and status in the same Cosmos document, relying on
application-level enforcement for directional integrity.

---

## Content Present in GCP HCP but Not in ARO HCP

| Topic | Description |
|-------|-------------|
| **Status reporting fields** | Documents `ObservedDesireUpdateTime` (all desires) and `AppliedResourceGeneration` (ApplyDesire only) for spec-to-status correlation |
| **Cooldown gate** | ApplyDesireController documents cooldown mechanism with `UpdateTime`-based change detection to avoid hot loops |
| **Firestore codec workaround** | `RawExtension` requires manual serialization via `firestore:"-"` tags because the Firestore Go SDK codec cannot handle `runtime.Object` |
| **`desirestatuswriter` create-or-replace** | Documented pattern for writing status to a separate database where the status document may not yet exist |
| **Authentication and isolation details** | Explicit IAM role/condition documentation for Workload Identity and per-database access |

### Questions for ARO HCP team

1. Does ARO HCP track an equivalent of `ObservedDesireUpdateTime`? If so, under what field name? Aligning on this would help keep the status contract consistent.
2. Does ARO HCP track `AppliedResourceGeneration`? The field lets the backend confirm a spec has been applied and the K8s object has advanced to the expected generation.
3. Does ARO HCP use a cooldown gate, or does Cosmos change feed semantics make it unnecessary?

---

## Content Present in ARO HCP but Not in GCP HCP

| Topic | Description | Action needed? |
|-------|-------------|----------------|
| **`KubeApplierDBClients` (plural) registry** | Backend's lazy per-MC client cache with `For(resourceID)` lookup and `ManagementClusterResourceIDs()` iterator | GCP constructs clients deterministically from MC name; may need a similar registry when the backend matures |
| **`UntypedCRUD` for orphan cleanup** | Cross-cutting cleanup via `UntypedCRUD(parentResourceID)` prefix walk | Not yet implemented on GCP; needs discussion on whether the Firestore model requires a different approach |
| **`ReadManyDesire` future consideration** | ARO mentions potentially adding list/watch support | GCP does not mention this; probably not needed unless a concrete use case arises |
| **`library-go/manifestclient`** | ARO uses this for fake K8s clients in unit tests | GCP uses standard client-go fakes; worth evaluating if manifestclient provides benefits |
| **Integration test setup reference** | ARO points to `test-integration/kube-applier/README.md` | GCP documents emulator-based testing but could expand setup instructions |

### Questions for ARO HCP team

1. How is orphan cleanup triggered on ARO HCP? Is it a controller, a periodic sweep, or on-demand? This will inform whether we need `UntypedCRUD` or can use Firestore's document lifecycle tied to MC teardown.
2. Has `ReadManyDesire` been implemented or is it still speculative?

---

## Interface Alignment

The Go interface stack is intentionally compatible:

| Interface | ARO HCP | GCP HCP | Notes |
|-----------|---------|---------|-------|
| `ResourceCRUD[T]` | `Get`, `List`, `Create`, `Replace`, `Delete` | `Get`, `List`, `Create`, `Replace`, `Delete` | Aligned |
| `SpecReader[T]` | N/A (same container, same CRUD) | `Get`, `List` (read-only, specs-db) | GCP-specific due to dual-database model |
| Optimistic concurrency | Cosmos ETag | Firestore `LastUpdateTime` precondition | Different mechanism, same semantics |
| Informers | Cosmos change feed | Firestore snapshot listeners | Different mechanism, same cache.SharedIndexInformer output |
| Listers | Per-type typed listers | Per-type typed listers | Aligned |
| Testing fakes | `listertesting.FakeCRUD` | `listertesting.FakeCRUD` with `UpdateTime` tracking | Compatible |

---

## Tense and Maturity

- **ARO HCP**: Partly aspirational ("will be", "will use"), suggesting the readme was written as a design spec before full implementation.
- **GCP HCP**: Present tense describing implemented and tested behavior. The code, tests, and readme are consistent.

---

## Recommended Discussion Topics

1. **`ObservedDesireUpdateTime` and `AppliedResourceGeneration`**: Should these be adopted as shared interface fields across both platforms?
2. **Orphan cleanup pattern**: GCP's dual-database model may simplify orphan cleanup (delete the MC's Firestore databases on teardown), but cross-desire orphan detection within a live MC still needs a pattern.
3. **Cooldown gate**: Is this useful for ARO HCP, or does Cosmos change feed behavior make it unnecessary?
4. **Status reporting contract**: Aligning on which status fields exist and what they mean would make it easier to share backend logic or monitoring across platforms.
