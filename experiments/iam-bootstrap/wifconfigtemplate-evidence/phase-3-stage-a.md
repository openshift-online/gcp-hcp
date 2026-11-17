# Phase 3 Stage A — code-first execution checkpoint

**Stage A code-first checkpoint complete, 2026-11-17.** Source was reviewed,
committed/pushed and natively built twice with matching checksums. This is a
development candidate, not a merge-ready CLI dependency update or Phase 3 live
completion. No live execution, assertion activation or Phase 4 is authorized.

## Scope and current work

- User approved Stage A only; stop before live GitOps, IAM mutation, assertion
  activation or hosted/customer allocation. User prefers retaining the eventual
  validated Cluster for Phase 4, under a separately approved deadline/cost agreement.
- CLI checkout `../gcp-hcp-ctl`, branch `feat/wifconfigtemplate-cli-phase3`, source
  `bf158abf4c9ae8935a52ac05139d43832577861c`, from
  `626da6d607c2529aee1ab87a9ccea212efe838c0`. Source was committed and pushed only
  to the selected fork `origin` (`apahim/gcp-hcp-ctl`); no upstream push or PR merge.
- External workspace:
  `/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-p3.go.work`.
  Maps CLI to current source and Gecko API/Orlop to clean
  `54709c471cdce451119eea69b69334224131ea48`; no local replace/go.work was committed.
- Implemented catalog pin/digest/availability checks, pure deterministic planner,
  actual default-worker identity, protected write-ahead journal and explicit resume,
  policy-v3/etag-safe deltas, resource/operation identity readback and owned cleanup.
- New standalone IAM create/verify/destroy and integrated/config-file creation paths
  use shared code; old embedded-role Manager is test-only. Owned network provisioning
  journals numeric IDs and LRO identities. Explicit config networks are read-only,
  never adopted/deleted. Legacy network commands refuse journal-marked stacks.
- Initial POST includes exact verified IAM application only after fresh readback,
  name/infra preflight and durable intent. Maintenance exclusions retain their
  reviewed start across midnight. Unknown submissions do not cause a second stack.

## Assurance and limits

Final `GOWORK=<recorded-workspace> make test` (`go test -race ./...`) and `make lint`
(`go vet ./...`) passed before commit, as did gofmt and `git diff HEAD --check`.
Architect review accepted the source commit as a **non-mergeable code-first
candidate**, conditional on published Gecko module alignment. Tests include the
golden server digest, fake cloud conflicts/crashes/partial outcomes, real local
public API validation/storage with its gate on/off, and zero mutation from previews.
All such API flag injection is confined to the local test process, not dev.

The final review's cleanup-realm blocker was fixed: the journal/fingerprint pins
the canonical credential-free API endpoint; teardown requires a fresh matching
Version UID at that endpoint, and explicit API Status absence rather than a
client-go-synthesized status from a proxy 404. Wrong realm/UID tests prove hosted
absence and cloud proof are not attempted. Live read-only absence compatibility
passed at 21:46:24Z; no API mutation was used to obtain it.

No direct private API imports/endpoints in production CLI. However the generated
public API package itself imports private conversion types transitively; the total
production dependency graph is not private-package-free. No server entry point was
added. Formal published module alignment/standalone release-CI support is pending;
the candidate requires the recorded external workspace.

Read-only runtime readback at **2026-11-17T21:45:21Z** reconfirmed eight Ready, zero
Cluster/NodePool/HC/HCP objects, 19/21 Version pins retained, template generation 7
and verification=false, assertion writes=false. Both contexts used GKE DNS endpoints.

Resolved ready-pod images remain exactly:

- API `quay.io/apahim/gecko:wif-p2-cel-54709c4-api` /
  `sha256:55df0ecd04b7b7eaebb6d80eb9021d431d803f71ab0cfb1be18fc3ef2b0daa35`.
- Seven controllers `quay.io/apahim/gecko:wif-p2-9f7161a-controllers` /
  `sha256:0c69927dc6976887987e20e6c7c1ce2c123fef8e3a4ed9fb6ce6ed55ff48b355`.

Fresh identity-token public catalog reads at 21:46:19Z confirmed `5.0.2` belongs
only to `candidate`, not `stable`; its UID/generation/pin and expected Available=False
are recorded in the proposal. An initial read-only membership assumption failed and
was corrected in the proposal; no alternate release was silently selected.

No live platform configuration, cloud IAM or hosted resource was mutated.

## Isolated native artifact and reproduction

- Source: `bf158abf4c9ae8935a52ac05139d43832577861c`, branch
  `feat/wifconfigtemplate-cli-phase3`, pushed to `apahim/gcp-hcp-ctl`.
- Toolchain: `go version go1.27.4 darwin/arm64`; `CGO_ENABLED=0`.
- Binary:
  `/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-p3-cli-bf158abf4c9a/gcphcpctl`.
- SHA-256: `eea76f687cb19883559f1923c14088fa73e0e9831f2ce63041f443134fbb6ec9`.
- Two independent outputs from the same committed source and fixed build stamp
  matched byte-for-byte. Both CLI and Gecko worktrees were checked clean before
  building. User-installed CLI was not replaced; no image/server build was needed.
- Local-only artifact checks passed: checksum, `version`, IAM create/verify/destroy
  help and Cluster create help. These invoked no cloud/API mutation or auth lookup.
- Adjacent local provenance: `build.json`, `SHA256SUMS`, `go-build-info.txt`,
  `smoke.json` and the second `gcphcpctl-repro-check` binary. These are temporary
  artifacts, not a release publication or committed binaries.

The external workspace uses Go 1.27.4 and exactly:

```go
use (
    /Users/asegundo/git/osd/gcp-hcp-ctl
    /Users/asegundo/git/osd/gecko/platform-api
    /Users/asegundo/git/osd/gecko/orlop
)
```

From the clean CLI source commit, the native build was:

```sh
GOWORK=/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-p3.go.work \
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
go build -mod=readonly -trimpath -buildvcs=true \
  -ldflags '-s -w -X github.com/openshift-online/gcp-hcp-ctl/pkg/cli.version=wif-phase3-bf158abf4c9a -X github.com/openshift-online/gcp-hcp-ctl/pkg/cli.commit=bf158abf4c9ae8935a52ac05139d43832577861c -X github.com/openshift-online/gcp-hcp-ctl/pkg/cli.date=2026-11-17T21:46:27Z' \
  -o /private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-p3-cli-bf158abf4c9a/gcphcpctl \
  ./cmd/gcphcpctl
```

Recreate the external workspace with the same source SHAs if temporary storage is
gone. Do not run the older published-module baseline, silently update dependencies,
commit machine paths or replace a global CLI. For another platform, build and
record a distinct artifact/checksum instead of reusing this one.

## Review disposition and conservative limitations

The required architect reviewed the initial design, WIP, final candidate and realm
fix. Final disposition: **no remaining source-commit blocker; non-mergeable until
published Gecko module alignment**. Final race/vet passed after the realm fix.

- Unknown POST + absent object has no sanctioned automatic release: no second POST,
  new infra ID, force unlock or automatic IAM/network deletion. Resources may remain
  allocated until a separately reviewed operator reconciliation/release.
- Unprovable create/delete operations (including expired LROs), changed role etags,
  missing/recreated worker identity and retired/replaced Version UID fail closed.
- Pool/provider identity has no immutable UID; ownership combines exact inputs,
  random operation marker and journal intent. Compute network deletion has no
  numeric-ID CAS. Independent reads narrow races, not close them against another
  authorized actor. Do not describe cleanup as atomic across API/cloud resources.
- The API realm proof's retained-Version requirement deliberately blocks cleanup
  after catalog retirement/recreation; this requires review, not a realm override.
- Protected state-file permissions/canonical paths are strict. Legacy output files
  do not establish ownership. Keep journals after cleanup as retired-ID evidence.
- Audience/mapping conventions remain outside schema-v1 digest; they are explicit
  desired fields bound into the plan/journal and verified from the provider.

The native candidate is not a live functional acceptance claim. No successful
installation, Node Ready, running upgrade or live teardown has been asserted.

Read-only customer-project observations and the unapproved live scope are in
[the Stage B proposal](phase-3-live-proposal.md). Costs, UTC retention deadline,
operator responsibility, CLI ADC authority and several runtime prerequisites still
need agreement/readback. Do not progress to live execution from this checkpoint.
