# Implementation Plan: GAR Registry Authentication via Workload Identity for Operator and Ignition Lookups

**Status**: Proposed — ready for implementation and live validation (revised design)
**Jira**: Spike [GCP-1491](https://redhat.atlassian.net/browse/GCP-1491) (this plan records its outcome) ·
Story [GCP-1490](https://redhat.atlassian.net/browse/GCP-1490) (closed; HO GAR lookup groundwork) ·
Epic [GCP-598](https://redhat.atlassian.net/browse/GCP-598) — Zero Egress for GCP HCP Platform Components
**Target repository**: `github.com/openshift/hypershift` (upstream — requires HyperShift maintainer review/sign-off)
**Date**: 2026-11-19 (original) · 2026-11-23 (linkage update)

**Related**: [zero-egress-worker-nodes](../design-decisions/infrastructure/zero-egress-worker-nodes.md)
(worker-side decision; explicitly defers control-plane authentication to this work) ·
Worker-side delivery: Epic [GCP-1268](https://redhat.atlassian.net/browse/GCP-1268) /
Feature [GCP-738](https://redhat.atlassian.net/browse/GCP-738)

## Objective

Allow the hypershift-operator (HO), control-plane-operator (CPO/HCCO), and ignition server to authenticate their
**registry API lookups** against Google Artifact Registry (GAR, `*.pkg.dev`) using GCP
Workload Identity (WIF) credentials, without requiring the HostedCluster pull secret to
carry GAR credentials. This enables a new GCP HostedCluster to resolve the OCP release image
(and the `hypershift`/CPO component image) in GAR through worker bootstrap, subject to the
separate kubelet image-pull prerequisites below.

## Scope

**In scope** (process-level registry API calls made by operators):
- HO: CPO image **labels lookup** (`GetControlPlaneOperatorImageLabels`), release payload
  resolution (`releaseinfo.Provider.Lookup`), payload arch/digest determination, ICSP/IDMS
  mirror availability checks.
- CPO: per-HCP release payload resolution (`buildReleaseProviders` → `UserReleaseProvider`
  / control-plane release provider), image metadata and release mirror checks.
- HCCO: release payload and image metadata resolution for guest-side config (in-place upgrades,
  node config).
- Ignition server: release metadata, image-references, MCO templates, and binary extraction
  during worker bootstrap.

**Out of scope** (solved by a separate initiative):
- Management-cluster kubelet image pulls (control-plane component pods) — GKE node SA / GAR
  credential provider handles this.
- Data-plane (guest worker) pulls and the HC pull secret contents.
- General support for non-GKE management clusters; the first version requires GKE Workload
  Identity and its pod metadata server.

## Recorded decisions

| Decision | Choice |
|---|---|
| Precedence for GAR hosts | **Pull secret first**, WIF fallback (explicit user creds win; WIF fills gaps) |
| Enablement | **HO flag, default off** (`--enable-gar-wif-auth`); direct HO → CPO options/env → HCCO and ignition env. No HC annotation or API changes. |
| Platform | GKE management cluster with Workload Identity configured and a pod metadata server. No ambient ADC discovery for registry credentials. |
| HO identity | Keep HO's existing management GSA (already used for PSC); grant that GSA GAR reader on the required repos. Do not replace its existing binding. |
| CPO/HCCO/ignition identity | Bind their *management-cluster* Kubernetes ServiceAccounts (KSAs) to a dedicated, repo-scoped registry-reader GSA. Use an explicit metadata-server token source and verify the expected GSA identity before use. This identity is available before the guest API exists. |
| Access boundary | All HCs managed by an enabled HO may use the GAR repos readable by HO; all per-HCP registry KSAs share the designated reader GSA. This shared access is intentional; keep IAM grants at repo scope. |
| Delivery | Land shared plumbing first if useful, but treat HO + CPO + HCCO + ignition support as one coordinated functional delivery. |

## Current state (verified)

Credential construction is centralized at exactly **three sites**, all building an
`auth.CredentialStore` from pull-secret bytes via `dockercredentials.NewFromBytes`:

| Site | File | Feeds |
|---|---|---|
| `GetRepoSetup` | `support/imageregistry/imagemetadata.go:377` | `getMetadata`, digests, manifests |
| `getRepository` | `support/imageregistry/imagemetadata.go:397` | `ImageMetadata`, `GetDigest`, mirror availability HEAD checks |
| repo client for release extraction | `support/releaseinfo/registryclient/client.go:276` | `ExtractImageFiles`, `ExtractImageFile`, `ExtractImageFilesToDir` → `RegistryClientProvider.Lookup` and ignition extraction |

These are the construction sites, **not** a complete list of call paths. In particular,
`ProviderWithOpenShiftImageRegistryOverridesDecorator.Lookup` performs a separate mirror
verification via `registryclient.GetRepoSetup` (`support/releaseinfo/registry_image_content_policies.go:34-51`).

Provider construction:
- HO: `NewCommonRegistryProvider` (`support/globalconfig/imagecontentsource.go:226`) builds
  `imageregistry.RegistryClientImageMetadataProvider` and the release decorator chain whose
  innermost delegate is `releaseinfo.RegistryClientProvider` (an empty struct today).
  The same metadata provider instance is passed to HC webhook setup
  (`SetupWebhookWithManager`, though the current validator does not perform a registry
  lookup) and to the NodePool reconciler (`hypershift-operator/main.go:847`).
- CPO: `buildReleaseProviders` (`control-plane-operator/main.go:290`) builds the chain with an
  inner `releaseinfo.RegistryClientProvider{}`.
- HCCO: `control-plane-operator/hostedclusterconfigoperator/cmd.go:285` builds its own chain.
- Ignition server: `ignition-server/cmd/start.go:183-208` constructs its own release and
  metadata providers; `ignition-server/controllers/local_ignitionprovider.go:356,767` and
  `ignition-server/controllers/image_file_cache.go:31` extract files independently.

Key facts that make this design cheap:
- The `auth.CredentialStore` interface is tiny: `Basic(url *url.URL) (username, password)`
  plus `RefreshTokenStore` (see `keyringCredentialStore` in
  `support/thirdparty/oc/pkg/cli/image/manifest/dockercredentials/credentials.go`).
- GAR accepts Docker basic auth with username `oauth2accesstoken` and an OAuth2 access token
  as password (scope `https://www.googleapis.com/auth/cloud-platform`). Exercise both basic
  and bearer-challenge flows in tests.
- CPO already has WIF in-process **for guest cloud APIs**, but it is *not* a bootstrap-ready
  registry identity: HO writes an `external_account` ADC
  config into the `control-plane-operator-creds` secret
  (`hypershift-operator/controllers/hostedcluster/internal/platform/gcp/gcp.go:332-344`,
  `support/gcputil/gcputil.go:337`), and the CPO deployment mounts it with
  `GOOGLE_APPLICATION_CREDENTIALS=/etc/gcp/application_default_credentials.json`
  (`control-plane-operator/controllers/hostedcontrolplane/v2/controlplaneoperator/deployment.go:283-312`).
  Its subject token (`/var/run/secrets/openshift/serviceaccount/token`) comes from a
  token-minter calling the **guest API** (`token-minter/tokenminter.go:204`). CPO must look
  up release metadata before creating that API (`hostedcontrolplane_controller.go:844`), so
  reusing this file would deadlock new-cluster bootstrap. Leave these cloud credentials intact.
- HO on GKE already has a management KSA → GSA binding documented in
  `docs/content/how-to/gcp/setup-management-cluster.md`; it needs a GAR repo IAM grant.
- The v2 framework creates component ServiceAccounts from embedded manifests; CPO, HCCO,
  and ignition each have their own KSA, so annotate all three for the registry GSA and bind
  each namespace/name to that GSA in GKE IAM.

## Design

### 1. New package `support/registryauth`

```go
// GARCredentials is an auth.CredentialStore backed by a GKE metadata-server
// token source. Construct it explicitly; do not use google.FindDefaultCredentials,
// which could select CPO's guest-dependent GOOGLE_APPLICATION_CREDENTIALS file
// or a node identity. Verify the metadata server's service-account email matches
// the configured GSA before accepting this source.
type GARCredentials struct {
    tokens oauth2.TokenSource
    // A per-store, concurrency-safe refresh-token store if required by the
    // Docker registry client's auth.CredentialStore interface.
}

// Exact API to be chosen during implementation; it must bind to the GKE pod
// metadata server, verify expectedGSA, and never fall back to ambient ADC.
func FromGKEMetadata(ctx context.Context, expectedGSA string) (*GARCredentials, error)
func Disabled() auth.CredentialStore
```

`Basic(url)` behavior:
- Return `("", "")` unless the parsed URL is HTTPS and its hostname is a
  subdomain ending in `.pkg.dev` (covering `REGION-docker.pkg.dev`). Reject the
  bare `pkg.dev`, deceptive suffixes, non-HTTPS URLs, and arbitrary token realms.
  `gcr.io` is out of scope even if backed by GAR.
- For GAR hosts: `("oauth2accesstoken", <current token>)` from the `TokenSource`
  (auto-refreshing). On token fetch failure: emit a rate-limited, token-free diagnostic
  and return empty creds; existing lookup errors then surface in reconciliation.
  Do not permanently disable WIF due to transient metadata/IAM errors: a later lookup
  must be able to recover without a restart. An identity mismatch must fail closed.
- Thread-safe: `TokenSource` must be safe for concurrent callers. Never log tokens or
  serialize them into Kubernetes objects.

`Layered(primary, fallback auth.CredentialStore)`: consults `primary` (the pull-secret
keyring) first; only when *both* username and password are empty consult `fallback`
(which self-scopes to GAR hosts). Do **not** retry failed explicit pull-secret credentials
with WIF; preserve pull-secret JSON parse failures. Explicitly implement/delegate
`RefreshToken` and `SetRefreshToken` for Docker bearer challenges. Keep any refresh-token
state isolated per composed registry context so different pull secrets cannot share it.

### 2. Thread the fallback store through the three call sites

- `imageregistry.RegistryClientImageMetadataProvider`: new exported field
  `FallbackCredentials auth.CredentialStore`. Thread through all methods and
  helpers including `ImageMetadata`/`getRepository`, `GetMetadata` (both branches),
  `getMetadataGetter`/`seekOverride`, `getOriginalImageTagDescriptor`, and
  `getRepoSetup`/`repoSetupFn`/`GetRepoSetup`:
  add `GetRepoSetupWithFallback(ctx, imageRef, pullSecret, fallback)` and keep
  `GetRepoSetup` as a wrapper passing `nil` (audit external callers first).
  Preserve the test hooks while proving production paths use the fallback.
- `releaseinfo.RegistryClientProvider`: new field `FallbackCredentials auth.CredentialStore`;
  thread through extraction and `registryclient.GetMetadata` / `GetRepoSetup`.
  Include `ExtractImageFile`, `ExtractImageFiles`, and `ExtractImageFilesToDir`
  (prefer new options-style variants over changing existing callers).
- `releaseinfo.ProviderWithOpenShiftImageRegistryOverridesDecorator`: add the fallback
  to its **separate** `repoSetup` mirror-verification call, not only the delegate.
- `NewCommonRegistryProvider`: new parameter (or settable field) for the fallback store,
  applied to the metadata provider, the innermost `RegistryClientProvider`, and
  the outer release-mirror decorator.
- Ignition's file cache must accept an injected fallback-aware extraction function;
  also wire its direct `ExtractImageFilesToDir` calls and both constructed providers.

Caches: do not cache OAuth tokens in metadata caches. Existing image metadata caches key
on image ref, release cache keys on release image, and mirror availability keys on mirror
ref + pull-secret hash. The flag and expected registry identity are fixed for the life
of a process. Verify existing caches still behave correctly on transient auth failure,
secret changes, and mirror recovery; token refresh occurs on *uncached* requests.

### 3. HO wiring (plumbing phase; not independently feature-complete)

- New flag `--enable-gar-wif-auth` (default `false`) in `hypershift-operator/main.go`
  `StartOptions`; expose it in `cmd/install/install.go` **and**
  `cmd/install/assets/hypershift_operator.go`. Add required, explicit
  `--gar-wif-ho-service-account` (HO's existing management GSA email) and
  `--gar-wif-registry-service-account` (dedicated registry GSA email) install/start
  options when enabled; validate both at startup. The latter is used for all three
  per-HCP KSAs. Document that a production deployment must set the HO option to
  the GSA already bound to HO for PSC.
- When enabled, configure HO's metadata-backed credential store with the explicit
  expected HO GSA identity (do not silently use node credentials). If the identity
  check fails, fail closed and clearly report the configuration error. Transient
  token acquisition failures must recover on retry.
- Set the store on the shared `registryProvider` (metadata, release, and mirror
  verification), also used by the NodePool reconciler.
- Pass enablement and registry GSA through `HostedClusterReconciler` into
  `ControlPlaneOperatorOptions` at the shared `reconcileControlPlaneOperator` call
  (covers both normal and legacy reconciliation). No HC annotation stamping.
- **Ops prerequisites (document, not code):** HO's KSA → existing HO GSA binding,
  repo-scoped `roles/artifactregistry.reader` on that GSA, and network access to
  the metadata server and GAR. Keep PSC privileges unchanged.

### 4. CPO wiring (functional phase)

- v2 `controlplaneoperator` component: set `ENABLE_GAR_WIF_AUTH=1` and the expected
  registry GSA on the CPO Deployment when enabled. Annotate its **management KSA**
  `iam.gke.io/gcp-service-account` with that GSA; when disabled, remove only
  operator-owned configuration and preserve unrelated SA annotations.
- `control-plane-operator/main.go`: when enabled on GCP, use explicit GKE metadata
  credentials and verify the expected registry GSA. Never use CPO's existing
  `GOOGLE_APPLICATION_CREDENTIALS` for registry auth. Wire both release providers,
  the release mirror decorator, and its metadata provider.
- IAM prerequisite (document): bind the management KSA in each HCP namespace to
  the registry GSA with `roles/iam.workloadIdentityUser` (GKE Workload Identity
  binding), and grant that GSA repo-scoped `roles/artifactregistry.reader`.

### 5. HCCO wiring (functional phase)

- v2 `configoperator`: propagate enablement and expected registry GSA from CPO's env
  and annotate the HCCO management KSA for the same registry GSA; add the IAM
  binding for that KSA. **Do not mount** `control-plane-operator-creds` or add
  `GOOGLE_APPLICATION_CREDENTIALS`: that file requires a guest token.
- `hostedclusterconfigoperator/cmd.go`: construct/verify the metadata-backed store
  and wire its release provider, release-mirror check, and metadata provider.

### 6. Ignition-server wiring (functional phase)

- v2 `ignitionserver` component: propagate enablement and expected GSA to the
  ignition-server Deployment; annotate its management KSA and bind it to the
  registry GSA. Do not reuse guest cloud credentials.
- `ignition-server/cmd/start.go`: construct/verify metadata-backed registry
  credentials for GCP when enabled and wire both release and image-metadata
  providers plus mirror verification.
- `ignition-server/controllers/local_ignitionprovider.go` and
  `ignition-server/controllers/image_file_cache.go`: route all direct extraction
  calls through the fallback-aware registry client. Include binaries and MCO
  templates, not just release image-references.

## File-by-file change list

| File | Change |
|---|---|
| `support/registryauth/registryauth.go` (new) | `GARCredentials`, explicit GKE metadata source, expected-GSA verification, `Disabled`, `Layered`, GAR host matcher |
| `support/registryauth/registryauth_test.go` (new) | see test plan |
| `support/imageregistry/imagemetadata.go` | `FallbackCredentials` field; cover direct, mirror and original-source metadata/digest/manifests paths |
| `support/imageregistry/*_test.go` | fallback threading tests |
| `support/releaseinfo/registryclient/client.go` | fallback-aware repo setup and all three file extraction functions |
| `support/releaseinfo/registryclient_provider.go` | `FallbackCredentials` field on `RegistryClientProvider` |
| `support/releaseinfo/registry_image_content_policies.go` | fallback on separate release-mirror verification path |
| `support/globalconfig/imagecontentsource.go` | `NewCommonRegistryProvider` configures metadata, release and mirror verification |
| `hypershift-operator/main.go` | default-off flag, expected identities/config, startup construction, provider/controller wiring |
| `hypershift-operator/controllers/hostedcluster/hostedcluster_controller.go` | pass options to shared CPO component reconciliation (normal and legacy) |
| `cmd/install/install.go`, `cmd/install/assets/hypershift_operator.go` | install flags, validation and deployment args |
| `control-plane-operator/controllers/hostedcontrolplane/v2/controlplaneoperator/{component,deployment}.go` | CPO management KSA GSA annotation and env propagation |
| `control-plane-operator/main.go` | metadata-backed registry store; both release providers, mirror and metadata wiring |
| `control-plane-operator/controllers/hostedcontrolplane/v2/configoperator/{component,deployment}.go` | HCCO management KSA annotation, env propagation (no guest creds mount) |
| `control-plane-operator/hostedclusterconfigoperator/cmd.go` | metadata-backed store on release, mirror and metadata paths |
| `control-plane-operator/controllers/hostedcontrolplane/v2/ignitionserver/{component,deployment}.go` | ignition management KSA annotation and env propagation |
| `ignition-server/cmd/start.go`, `ignition-server/controllers/{local_ignitionprovider,image_file_cache}.go` | ignition registry credentials and all extraction paths |
| `docs/content/how-to/gcp/...` (+ `docs/mkdocs.yml` nav if a page is added) | GKE/WI prerequisites, HO and per-HCP KSA bindings, GSA IAM, precedence, troubleshooting and kubelet pull requirements |

## Test plan

- `support/registryauth` (unit): host matching table (`us-central1-docker.pkg.dev` ✅,
  `quay.io` ❌, `gcr.io` ❌, bare `pkg.dev` ❌, `evilpkg.dev` ❌,
  `foo.pkg.dev.evil.example` ❌, HTTP ❌, wrong bearer realm ❌); layering precedence
  (both primary fields empty to fall back; incomplete/invalid explicit credentials
  must not fall back); invalid pull-secret JSON; refresh-token isolation; concurrent
  requests; token expiry/refresh, transient failure recovery, identity mismatch,
  `Disabled()` no-op and redacted, rate-limited errors.
- **Require** an httptest registry with realistic basic and bearer 401 challenges;
  verify token reaches manifests and blob extraction. Store-only tests do not
  prove that the registry client sends the expected Authorization header.
- `support/imageregistry`: exercise `ImageMetadata`, `GetManifest`, `GetDigest`,
  `GetMetadata` with and without overrides; mirror availability, original-tag
  fallback, and error recovery. Use existing `repoSetupFn`/`metadataGetter` hooks
  where helpful, but also exercise production paths end to end.
- `support/releaseinfo`: extraction of single file, file list and directory;
  outer mirror verification with fallback; preserve behavior of old signatures.
- HO/controller/install tests: flags default off and invalid/missing GSA rejected;
  enabled propagation for normal and legacy paths; no registry metadata-service
  calls when disabled; no HC annotation changes.
- v2 component tests: CPO/HCCO/ignition env, expected GSA and management KSA
  annotation on/off, platform gating, and preservation of unrelated SA annotations.
  Exercise both operator startup identity mismatch and transient metadata failure.
- Verify that for identical HC/NodePool inputs the rollout hashes do not change;
  registry credentials must not enter ignition/MachineConfig/hash inputs.
- Live validation (fresh GKE hosted cluster):
  1. Prepare GAR repos. Grant `roles/artifactregistry.reader` at repo scope to
     HO's existing management GSA and the dedicated registry GSA. Preconfigure
     GKE Workload Identity IAM bindings for the predictable HCP namespace and
     CPO/HCCO/ignition KSA names (the KSAs themselves will be created during
     reconciliation); allow for IAM propagation and retry. Ensure management/guest
     kubelet image pulls have their own required authentication.
  2. Enable HO with both explicit GSA options; create a GCP HC whose
     release (and relevant `hypershift` component) image is in GAR, with **no GAR
     entry** in the HC pull secret. Do not assume the guest API already exists.
  3. Verify HO release and labels, CPO bootstrap/release lookup, HCCO, ignition
     metadata/file extraction and worker readiness. Exercise an uncached lookup
     after a token refresh, or advance time in automated tests.
  4. Negative cases: disabled flag, wrong GSA binding, IAM denied, transient
     metadata outage, explicit bad pull-secret credentials, and non-GAR hosts.
  5. Existing `e2e-gke` has GKE + WIF (see
     `docs/content/how-to/gcp/e2e-gke-ci-job.md`), but GAR provisioning and
     registry-reader IAM fixtures are not confirmed. Investigate adding those
     to CI; until then treat this checklist as required manual acceptance.

## Rollout & compatibility

- Flag default off → no intentional registry behavior change; no API/CRD or HC
  annotation changes. Enabled configuration is limited to operator Deployments
  and their management-cluster ServiceAccounts.
- Version skew: old CPO/HCCO/ignition binaries will not gain this feature from
  new env vars or KSA annotations alone. Declare minimum supported CPO release
  containing all three per-HCP code paths and coordinate rollout; older HO
  simply has no opt-in flag.
- **No intended NodePool config-hash impact**: registry identities must not flow
  into ignition/MachineConfigs or rollout hash inputs. Verify unchanged hashes
  with tests; adding registry auth to the ignition *process* is not the same as
  changing the ignition *payload*. Management workloads may roll out when enabled.
- Failure UX: with the flag off and no pull-secret GAR creds, behavior is exactly today's
  (lookup errors in existing conditions). With the flag on and bad IAM, errors surface in the
  same conditions with the image ref included.

## Verification

- Targeted `go test -race` for touched packages (`support/registryauth`,
  `support/imageregistry`, `support/releaseinfo`, `hypershift-operator`,
  `control-plane-operator`, `ignition-server`, `cmd/install`) per `DEVELOPMENT.md`.
- `make lint` and `make verify` (includes hypershiftlinter; no status-patching changes
  expected — no HC/HCP status writes added beyond existing patterns).

## Implementation checks (not product decisions)

1. Confirm GKE pod metadata reports the annotated GSA email for all four workloads
   and that the expected-GSA check does not inadvertently accept the node GSA.
   Prefer an explicit, context-bounded identity probe; do not mask failures as
   successful registry authentication.
2. Confirm the exact HTTPS GAR bearer-challenge realm(s) with a real GAR request.
   Only send the OAuth token to a validated GAR endpoint; reject untrusted realms.
3. Confirm the minimum OCP/CPO release carrying CPO/HCCO/ignition changes and
   document the skew/upgrade sequence before enabling HO on existing clusters.
4. Confirm whether `e2e-gke` can provision GAR repos/IAM and run a private-release
   fixture; otherwise record the live manual acceptance evidence.
