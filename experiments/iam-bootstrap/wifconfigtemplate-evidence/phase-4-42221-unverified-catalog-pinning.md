# Phase 4 — unverified 4.22 catalog discovery and immutable pinning

**2026-11-20, final public/guest/management readback 10:49:52 UTC. This separately
approved catalog-only slice passed; Phase 4 remains INCOMPLETE.** No template
was verified, assertion writes were not enabled, and no IAM, customer resource,
installation, upgrade, guard rehearsal, migration replay or teardown ran.

## Scope, review and staging

- Selected exact source **4.22.21** payload
  `quay.io/openshift-release-dev/ocp-release@sha256:828fa71fe839d280cd731271d04917c5a5ca7f7ce49dd68e3eb1a5d37f9f7f02`
  and target **5.0.0** payload
  `quay.io/openshift-release-dev/ocp-release@sha256:5d7ed8b3e98bc3a10c50f36f07262a962d59ad2d3ea5fb4935ff10c408c02e55`.
  The earlier read-only graph proved the direct candidate-5.0 edge; this step
  did not request or validate an actual upgrade.
- Gecko source/image **`6c28cf4`** and local, unpushed CLI **`c3daed0`** were
  unchanged. The actual native `a727640` CLI artifact retained checksum
  `bba7d96f0fef9a79581d7a49b7354114b0e0f5dcdb842e063b6541acbbbeb13e`.
  No images were built or published. Nine Deployments, including the enabled
  NodePool-upgrade coordinator, remained the expected footprint.
- Infra checkout `/Users/asegundo/git/osd/gcp-hcp-infra-dev-main`, clean before
  this step, fast-forward published to **`origin/dev-apahim`** in two separate
  commits with remote SHA readback:
  1. **`39b3728f4a25d9650c2c80c43a9bfc9478dbeba1`** added only the
     `4.22.0`, `verified: false` entry to the personal-dev API override and
     generated Application. Root and API child Synced/Healthy before stage two;
     new template UID `6ece8281-a523-40d8-9787-78de5fde1dfc`, generation 1.
     Existing 21 Version specs/19 pins and retained stack remained unchanged.
  2. **`6ce465af529b3d926dccb23efd12ea40a70aba2d`** changed only the
     personal-dev candidate `minimumSupportedVersion` from `5.0` to `4.22`
     in the version-sync override/render. The actual running version-sync source
     stayed `9f7161a`, without image or Deployment changes. The install default
     remained `5.0.0`, fleet minor `5.0`; fast/stable/eus unchanged.
- Before each push, actual live root Helm values were used to compare all **23**
  child Application specs. Each stage changed precisely one child. Stage-one
  child Helm render added only one unverified template to its 26 existing
  objects: existing objects, including the **byte-identical completed migration
  Job**, were unchanged. The two template specs were identical and use the same
  reviewed executable contract digest
  `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`.
  Stage-two version-sync child render had eight objects and only the candidate
  Channel minimum changed. Spanner, ESP, `ApplyOutOfSyncOnly=true`, API
  `prune=false`, migration image pin and both closed gates were preserved.
- Renderer ran twice for each stage using the dedicated Python venv, with
  identical second-pass output. `git diff --check` and Argo script tests (54,
  two optional skips) passed. The required architect review gave go for stage
  one and a conditional go for stage two after template-first readback, then
  separately reviewed the actual stage-two two-file diff and gave go. Its
  earlier suggestion that the candidate default condition should be True was
  corrected: it must remain **False** while 5.0.0 is unverified. No source
  application code, schema or chart was modified; prior Gecko tests/builds are
  recorded in the bridge code/rollout checkpoint and were not repeated here.
- Only the regional **root** Application cache was normally refreshed after
  each publication. Ordinary Argo automated reconciliation applied the changes;
  no manual sync, direct resource apply, Force/Replace or Job replay. Stage-two
  root/child were Synced/Healthy; source minimum became `4.22` by 10:48:10 UTC;
  version-sync naturally reached the expected snapshot by 10:48:58 UTC.

## Discovery and independent acceptance

- Fresh upstream read-only shadow at **10:46:37 UTC** before stage two predicted
  60 desired Versions: **39 additions** (36 4.22 and three 4.23), **no removals**
  and **no change** to any existing release payload/channel membership. This
  explicitly approved footprint is larger than the single selected source
  release. After sync there were exactly **60 Versions / 55 pins**.
- All original **21 Version UIDs and full specs**, including all **19 original
  references**, remained exactly equal to their pre-rollout snapshot. Exactly
  36 new 4.22 Versions have `name=4.22.0`, UID
  `6ece8281-a523-40d8-9787-78de5fde1dfc` and the digest above, and current
  `Available=False/WIFConfigTemplateUnverified`. The three 4.23 Versions have
  **no reference** and current `Available=False/WIFConfigTemplateNotFound`.
  There was no 4.23 boundary or eligibility fallback.
- Exact `4.22.21` Version UID `d35674b4-3032-4c99-906d-82222872844a`,
  generation 1, has the selected payload, candidate membership and the new
  immutable 4.22.0 reference. Exact `5.0.0` retains Version UID
  `9bbbd3d4-d2c6-442c-942d-4c65c3c873a5`, generation 2, original selected
  payload, original template UID
  `fb623828-d923-4540-82f1-df5c01503823` and same digest; both selected
  releases remain unavailable because both templates are **verified=false**.
  Candidate `DefaultVersionAvailable=False/DefaultVersionUnavailable` remains
  current to the changed Channel generation, not Unknown. Both release and
  template public HTTPS GETs returned 200 with those identities/statuses.
- Independent private readback **10:49:32 UTC** found one original Cluster
  `wif3-first` UID `6b152684-797c-4f17-a258-43538f8013c8`, generation 2,
  and one original NodePool `workers` UID
  `a8768aef-c352-47bb-9f32-3638275f1e15`, generation 1: original specs,
  Dispatched 5.0.2 receipts, Complete requirements and no upgrade request or
  ControlPlaneUpgrade. Nine Deployments updated/Ready (1/1), both gates false;
  sampled Ready pod containers had zero restarts. The coordinator remained
  enabled; this is **not** an upgrade hold. The root, API, version-sync and
  coordinator Applications were Synced/Healthy at their expected source refs.
- Original migration Job UID `daa14792-1593-4a92-abe8-2b8f32ba3fd1`, spec
  digest `273c6c47a0da1d9383178425ca07b88422a6c29bd3a8bd26cdd70d8e5fae93ae`
  and Complete/Synced state stayed intact. Spanner UID
  `99e4163c-79f9-4875-92b9-0e576b9a5927`, spec and Ready state remained;
  API still selected Spanner. Management HyperShift Application remained
  Synced/Healthy at CRD-only source
  `914334ae61352fa5641e3f1d50250e87299e9eb3` and frozen annotation
  `3d8a55cb8c633329d2a64939e7e0ab20520107ee`.
- All three protected journal SHA-256 values remained unchanged: primary
  `6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995`,
  retired v1 guard
  `af1f4abf8e822266f8a089bf03773afb8eb908083ca31f28f19e1a025d178588`,
  completed v2 guard
  `54ec587d465eed4e5749dd23f8becd42e86018a61a06d9217412bb1866b8cc36`.
  No primary update sidecar. Previous guards were **not rerun**.
- Public TLS Cluster GET 200 preserved the original public spec digest
  `d8d739a85da263b872e1d20c8189aa7a0a1440e213adebb63a9f9f7468669a26`.
  At **10:49:52 UTC**, actual guest CVO still reported 5.0.2 Completed,
  Available=True, Failing=False, Progressing=False, with original single Ready
  Node UID `79b85ccd-768f-44f9-bc5b-f40a2bdd2cab`. Management HC/HCP and
  NodePool also remained healthy at 5.0.2. Bounded five-minute version-sync
  log sample contained 39 creation lines/one fetch summary and no sampled
  error/panic/forbidden matches; coordinator sample had no upgrade-write match.
  These samples are not sustained monitoring proof.

## Hard next boundary

**Phase 4 is INCOMPLETE.** Keep both templates unverified and assertion writes
false. Do not infer fresh 4.22 IAM compatibility from an identical executable
digest, or hard same-project resource authorization isolation from project-wide
roles/shared default worker SA. The second installation's footprint/cost/time,
IAM/cloud readback and new assertion, shared verification, actual upgrade and
cleanup all need separate review/approval. No separate CPO credential inspection
is a prerequisite; diagnose a concrete failure during approved normal pulls.

Before opening **shared `5.0.0` eligibility**, explicitly protect retained
`wif3-first`: its current maintenance exclusion expired 2026-11-19 and it
already asserts the 5.0.0 contract. A new bounded exclusion needs separate
approval and protects ordinary automatic upgrades, **not** customer requests
or applicable overrides. Review a true no-upgrade-write boundary and confirm
no accepted preparation/upgrade or customer request before verification.
Customer upgrade requests bypass maintenance exclusions and a future target
assertion could immediately release previously blocked work. Do not silently
change the retained spec or assume a closed assertion-write gate stops an
already-recorded assertion. Keep the dev-only coordinator's broad DNS egress
and excess main-resource NodePool patch RBAC out of broader promotion.

Do not casually restore candidate's old floor: version-sync would age/delete
the newly discovered Versions; the retained 4.22.0 template cannot simply be
deleted. Any recovery must first inspect consumers, pins and state and have a
separate reviewed scope. Preserve Spanner, original migration Job/image and CRD
source/stamp; a rollback of a catalog floor is not an upgrade rollback.
