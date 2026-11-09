# gitops-bootstrap renderer

A lean, portable, deterministic, credential-free Helm renderer for the
OpenShift GitOps operator's CRDs and operator manifests. It produces the
`crds.yaml` and `operator.yaml` that a bootstrap bundle later wires into
Config Sync — it does not itself apply, publish, or own any bootstrap
rollout.

This is adapted from the retained GCP-1226 research prototype at
`gitops-operator/prototype/update.sh` (outside this repo). That prototype is
read-only research input: it hardcodes a developer chart registry
(`quay.io/anjoseph/...`) and developer identities, and is explicitly not
deployable product tooling. Nothing here modifies or executes that
prototype.

## Scope

This is GCP-1280 engineering work on the portable renderer itself. Resolving
a reviewed, deployable chart/image tuple is separate, ongoing GCP-1280
engineering work, not something this renderer's existence skips; GCP-1286
separately owns certifying a chosen release as supported — neither gate is
satisfied by anything in this directory.

In scope here:

- `render.sh` — the renderer itself.
- A pin/input contract (`pins.example.yaml`) describing what a real pins
  file must contain.
- The kustomize operator overlay (`overlay/`) that sets the release
  namespace Helm doesn't emit.
- Focused offline checks (`tests/run-tests.sh`) against synthetic fixtures
  (`testdata/`).

Beside the renderer, GCP-1280 Step 2 adds (see "Reviewed tuple" below): the
reviewed, checksum-pinned chart archives and allowlist (`pins.yaml`, `charts/`,
`allowed-images.txt`, provenance in `chart-src/PROVENANCE.md`) and
`assemble.sh`, which composes inactive MC/region bundles from this renderer's
output (`bootstrap-gitops/README.md`).

Out of scope for the renderer itself (left for later GCP-1280/GCP-1286/GCP-1289
work):

- Deriving namespace/name identities from an MC/region naming scheme. This
  renderer takes them as flat, explicit flags (`--namespace`,
  `--instance-namespace`, `--service-account`, `--image-pull-secret-name`).
  Per-target composition and its naming/collision validation live in
  `assemble.sh`, not here.
- Any change to `bootstrap/`, parent/child RootSync manifests, or migration
  control flow.
- Any supported/certified claim about a chosen release — that gate belongs
  to GCP-1286.

## Usage

```sh
./render.sh \
  --pins pins.yaml \
  --output-dir /path/to/new-output-dir \
  --namespace gitops-operator \
  --instance-namespace argocd \
  --service-account openshift-gitops-operator-controller-manager \
  --image-pull-secret-name redhat-registry-pull-secret \
  --allowed-image <repo>@sha256:<digest> \
  [--allowed-image <repo>@sha256:<digest>]... \
  [--image-allowlist allowed-images.txt]
```

Requires `helm`, `kubectl` (for `kubectl kustomize`), `yq`, and `jq` on
`PATH`. Never contacts a network or a cluster: chart inputs must already be
local, vendored archives.

`--output-dir` must not already exist; the renderer fails closed rather than
overwrite anything.

At least one `--allowed-image` and/or a non-empty `--image-allowlist` entry
is **required**. There is no default, and no way to render with the check
disabled, omitted, or pointed at an empty file — all three are explicitly
rejected.

### Pin/input contract

`render.sh` takes a `--pins` YAML file naming two local Helm chart archives
(the CRDs chart and the operator chart) plus their expected `version` and
`checksum`. See `pins.example.yaml` for the schema and field semantics.
`checksum` is always required and always verified against the local archive
byte-for-byte; `origin`/`digest` are preserved provenance metadata and may
be `null` if not yet verified, but are never invented.

### Image handling

Every image-shaped value in the rendered operator manifest must match the
same conservative, fully-qualified image-reference grammar **and** be
present in the mandatory `--allowed-image`/`--image-allowlist` set, or the
render fails closed. Exactly one function (`valid_image_ref` in
`render.sh`) implements this grammar and is used for both allowlist entries
and every rendered value, so a given string either passes both checks or
neither. The grammar is:

```
host[:port] ( "/" repo-component )+ "@sha256:" <64 lowercase hex>
```

- `host[:port]`: one or more dot-separated DNS hostname labels, each
  `[A-Za-z0-9]` with internal `-` allowed but never leading or trailing a
  label, optionally followed by `:<port>`. Modeled on the domain grammar in
  the Docker/OCI distribution reference spec
  (`domainNameComponent`/`domainName` in
  [`regexp.go`](https://github.com/distribution/reference/blob/main/regexp.go)).
  Mixed case is accepted because DNS hostnames are case-insensitive and
  registries are commonly referenced that way (e.g. `Quay.io`); consecutive,
  leading, or trailing dots, a label starting/ending in `-`, underscores,
  whitespace, and any other character are all rejected (e.g. `quay..io` and
  `quay-.io` are not valid registry hostnames). IPv6 and other host forms
  outside this labeled-hostname subset are not supported.
- `repo-component` (one or more, each preceded by `/`): lowercase
  alphanumeric runs separated only by a single `.`, a single or double `_`,
  or a run of one-or-more `-` — e.g. `my-repo`, `my_repo`, `my__repo`,
  `my.repo`, `my---repo` are all valid components. This mirrors the
  Docker/OCI distribution reference spec's path-component grammar.
  **Uppercase letters, a literal `..`, and a run of three-or-more `_` are
  all invalid and rejected** (e.g. `EXAMPLE`, `ex..ample`, `ex___ample`).
- digest: exactly `@sha256:` followed by 64 lowercase hex characters.

Examples that pass: `quay.io/example/foo@sha256:<64 hex>`,
`registry.example.com:5000/ns/foo@sha256:<64 hex>`,
`quay.io/my-example_repo__sub/operator-two@sha256:<64 hex>`.

This deliberately does NOT support: a bare `@sha256:...` with no repository;
a single unqualified segment (implicit `docker.io/library/...`, e.g.
`nginx@sha256:...`); a floating tag, with or without a digest (e.g.
`foo:latest` or `foo:latest@sha256:...`); or any registry/path syntax beyond
this explicit subset (no IPv4 hosts, no tag-only references, etc.) — this
tool's known chart inputs don't need them, and no parsing
library/dependency is introduced to support them.

This check covers:

- every container/initContainer `image:` field (the manager image and any
  others the chart renders), and
- every environment variable on those containers whose name contains the
  uppercase string `IMAGE` (case-sensitive, anywhere in the name) — the
  conventional way Operator-SDK-style charts (including this one) pass operand
  image overrides, e.g. `ARGOCD_IMAGE`, `ARGOCD_REPOSERVER_IMAGE`,
  `ARGOCD_DEX_IMAGE`, `BACKEND_IMAGE`, `RELATED_IMAGE_*` and
  `GITOPS_CONSOLE_PLUGIN_IMAGE_PF5`. Such an env var must set a literal
  `value` that is an approved digest-pinned reference; one set via `valueFrom`
  cannot be verified statically and is rejected. Names without uppercase
  `IMAGE` (including lowercase variants) are not inspected, and a non-image
  variable whose name contains `IMAGE` (for example `IMAGE_PULL_POLICY`) is
  rejected rather than ignored.

This tool ships no default allowlist and bakes in no default image pin —
there is no bypass for omitting the check, and an empty `--image-allowlist`
file is treated the same as omitting it.

### Reviewed tuple (GCP-1280 Step 2)

The renderer itself still ships no default image allowlist and bakes in no
image pin. The reviewed OpenShift GitOps 1.22.2 tuple is supplied as inputs
beside it: locally maintained, checksum-pinned chart archives (`pins.yaml`,
`charts/`) authored from the official operator bundle, and the reviewed
allowlist (`allowed-images.txt`). Their provenance, digests and the documented
differences from the official bundle are in `chart-src/PROVENANCE.md`.
`assemble.sh` composes the inactive MC/region bundles from this output (see
`bootstrap-gitops/README.md`); no assembled bundle or shared-CRD artifact is
committed, and its tests (`tests/run-step2-tests.sh`) assemble both types fresh
and compare them with `tests/golden.sha256`. `.gitattributes` marks only the
chart archives (`charts/*.tgz`) as `linguist-generated`; the generated chart
`rbac.yaml`, `values.yaml` and `allowed-images.txt` stay visible because they are
review gates.
Resolving usable pins is engineering work; GCP-1286 separately owns certifying
a chosen release as supported, and nothing here is that certification.
`assemble.sh` rechecks every `IMAGE`-named env value and container image on the
final composed bundle, and (with `python3`, via `check-depends-on.py`) validates
the bundle's Config Sync `depends-on` graph.

### What this renderer guarantees about its output

Enforced by validating the rendered YAML itself (not just that the expected
Helm flags were passed — a chart that ignores one of these value paths
fails closed rather than silently producing unreviewed output):

- `--include-crds` is always passed to `helm template` for the CRDs chart,
  and the resulting `crds.yaml` must contain `CustomResourceDefinition`
  objects only — a chart that smuggles another object into `templates/`
  (not excluded from `helm template` the way `crds/` is) is rejected before
  publication.
- Only the top-level `status:` key is stripped from each rendered object
  (Config Sync rejects a declared top-level status); nested schema
  properties literally named `status` (e.g. inside a CRD's
  `openAPIV3Schema`) are left untouched. The renderer fails closed if
  stripping changes anything beyond that top-level key.
- `pullSecret.create=false` is always set and verified; no dockerconfig data
  or credential-bearing Secret is ever rendered. A Secret of type
  `kubernetes.io/service-account-token` is permitted only with empty
  `data`/`stringData` (Kubernetes populates the token at apply time, not
  this renderer), and if its `kubernetes.io/service-account.name` annotation
  is set, it must match `--service-account`.
- `--service-account` must be declared as a `ServiceAccount` in the output,
  and every `Deployment` must reference it as `serviceAccountName`. Names
  follow the DNS-1123 **subdomain** rule Kubernetes actually applies to most
  object names (dot-separated DNS-1123 labels, max 253 chars) — unlike
  Namespace names, dots are valid (e.g. `foo.bar-baz`).
- `ENABLE_CONVERSION_WEBHOOK=false`, `ARGOCD_CLUSTER_CONFIG_NAMESPACES` (set
  to `--instance-namespace`), and `--image-pull-secret-name` are all
  verified against the one Deployment that owns the container named
  `manager` (the prototype's name for the operator's main controller
  container) — specifically: the env values are checked on that container,
  and `imagePullSecrets` is checked on that same Deployment's pod spec. A
  sidecar/initContainer carrying the right env, or a wholly separate
  Deployment carrying the right `imagePullSecrets`, does not satisfy these
  checks. The render fails closed if no Deployment, or more than one
  Deployment, owns a container named `manager`, or if that Deployment
  doesn't contain exactly one such container.
- `operator.yaml` must render at least one `Deployment`, and every object
  kind it contains must be one of: `Namespace`, `Deployment`,
  `ServiceAccount`, `Service`, `Role`, `RoleBinding`, `ClusterRole`,
  `ClusterRoleBinding`, `Secret`, `ConfigMap`. Any other kind fails the
  render rather than being published unreviewed; extending this list for a
  real chart that legitimately needs another kind is a deliberate contract
  change, not a default to relax quietly.
- All namespaced operator objects are explicitly scoped to `--namespace`;
  `ClusterRole`/`ClusterRoleBinding` must not declare a namespace.

## Offline checks

```sh
./tests/run-tests.sh
```

Builds several synthetic Helm chart fixtures from `testdata/` with `helm
package`, exercises `render.sh` (which calls real `helm` and `kubectl
kustomize`, not mocks) against both a happy path and deliberately broken
inputs, and asserts on the rendered output. Covers: repeatable/deterministic
output, checksum and chart-version pin rejection, nested-vs-top-level status
handling, CRD-chart contamination with a non-CRD object, floating/malformed
manager and operand image rejection, unresolved `valueFrom` image env
rejection, the mandatory (non-bypassable, non-empty) image allowlist,
unsupported object kind rejection, a missing/inconsistent ServiceAccount, an
over-length namespace, no overwrite of an existing output directory, and the
absence of any credential-bearing object.

Image-reference grammar coverage (exercised in both an `--allowed-image`
entry and a rendered value, not just the regex in isolation): a bare
`@sha256:...` digest with no repository, an uppercase repository path, a
double-dot repository path, a triple-underscore repository path, a
double-dot host, and a host label starting with `-` (all rejected); a
fully-qualified `registry:port/path` reference, a repository path combining
a dash-run, a single underscore, and a double underscore, and a mixed-case
registry host (all accepted, with the rendered value asserted to round-trip
exactly). The last few host/path-only cases reuse one
`operator-chart-image-override-src` fixture, transformed per case at test
time (`package_chart_with_image`) rather than adding a new fixture source
tree per image value.

Manager-identity and postcondition-binding coverage: a missing `manager`
container or two `manager` containers in one Deployment, a decoy
sidecar/initContainer carrying the correct env while the manager container
does not, and a decoy Deployment carrying the correct `imagePullSecrets`
while the manager's own Deployment does not (rejected) alongside the
ordinary case of the manager's own Deployment carrying it correctly next to
an unrelated decoy Deployment (accepted).

DNS-1123 subdomain coverage: a dotted ServiceAccount name (positive)
alongside malformed dot placement — leading dot, trailing dot, double dot,
and uppercase (negative).

Contacts no network and no cluster.

## Controller namespace role (GCP-1285)

`render.sh` and `assemble.sh` accept the optional
`--controller-cluster-role NAME` (DNS-1123 subdomain, maximum 253 characters).
It sets and verifies the unique manager container's `CONTROLLER_CLUSTER_ROLE`
input. Omitting it preserves the original output. The reviewed cluster owner
must supply that ClusterRole separately. This input replaces default namespace
roles; it does not remove default cluster-wide or interim parity bindings, grant
cloud access, or establish an admission-enforced permission boundary. Coordinate
it with the reviewed ArgoCD policy and verify the effective binding inventory.
