# Phase 3 Stage B — read-only preflight, blocked

**Observed 2026-11-18, 10:53–10:55 UTC.** The user requested execution of the
next-phase plan, whose first checkpoint is read-only refresh followed by separate
approval of an exact live manifest. No allocation, cleanup smoke or gate activation
was attempted. **Stop: the candidate CLI has no Google Cloud ADC credentials.**

## Source and isolated artifact

- CLI clean at `bf158abf4c9ae8935a52ac05139d43832577861c`, branch
  `feat/wifconfigtemplate-cli-phase3`.
- Gecko clean at `54709c471cdce451119eea69b69334224131ea48`.
- Dev infra clean at `327ebd9f10a761683d5b3fb5b4526bc4a7033bd3`.
- Original infra checkout's unrelated Spanner/variables/Endpoints edits and PDF
  remain untouched. Existing untracked gcp-hcp files were not staged or overwritten.
- Isolated native candidate checksum revalidated:
  `eea76f687cb19883559f1923c14088fa73e0e9831f2ce63041f443134fbb6ec9`.
  Path and build provenance remain in [the Stage A handoff](phase-3-stage-a.md).
- No source edits, rebuilds, module changes, image pushes, rollout or Terraform apply.

## Runtime readback

Using the two established GKE DNS endpoint contexts at 10:53:44Z:

- Eight Gecko/API Deployments Ready, each 1/1.
- API digest remains
  `sha256:55df0ecd04b7b7eaebb6d80eb9021d431d803f71ab0cfb1be18fc3ef2b0daa35`.
- Seven-controller digest remains
  `sha256:0c69927dc6976887987e20e6c7c1ce2c123fef8e3a4ed9fb6ce6ed55ff48b355`.
- API assertion writes explicitly false; template `5.0.0` verified=false, generation 7,
  UID `fb623828-d923-4540-82f1-df5c01503823`.
- 21 Versions, 19 retained pins; zero Cluster, NodePool, HC and HCP objects.
- Management HyperShift operator Ready 2/2; kube-applier-gcp Ready 1/1.
  Their readiness is not proof that a future CAPG/CPO installation or guest access works.

Fresh identity-token public reads at 10:54:40Z confirmed:

- `5.0.2` is still in **candidate only**, UID
  `a85eeff8-46e2-4876-b04e-87355a132a3d`, generation 2.
- Its retained reference still selects `5.0.0` and digest
  `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`.
- Available=False/WIFConfigTemplateUnverified is expected while the live gate is closed.
- The customer namespace is readable and contains zero Clusters. Proposed name
  `wif3-first` still returns explicit API Status/Failure 404 (not a proxy bare 404).

## Discovery and credential blocker

The unqualified shared-dev discovery hostname does not resolve. The correct
personal-dev manifest was fetched successfully, without changing DNS or discovery:

`https://discovery.apahim1110.dev.gcp-hcp.devshift.net/v1/regions.json`

For `northamerica-northeast2` it advertises:

- API: `https://platform-api-northamerica-northeast2-apahim1110.dev.gcp-hcp.devshift.net`
- OIDC base: `https://oidc.dev-reg-na-ne2-apahim1110.dev.gcp-hcp.devshift.net`

Use CLI `--environment apahim1110.dev`, not bare `dev` for this personal footprint.

Credential-metadata-only inspection found no ADC file; the active gcloud account
is `asegundo@redhat.com`. No credential contents were printed or persisted in evidence.
The verified Stage A binary was invoked only with the read-only flow:

```sh
gcphcpctl cluster create wif3-first \
  --setup-infra --infra-id wif3-7d9e \
  --version 5.0.2 --channel-group candidate \
  --project apahim-dev-test-1 --region northamerica-northeast2 \
  --environment apahim1110.dev --vpc-cidr 10.0.0.0/24 --dry-run --v=0
```

It exited 1, reporting missing ADC for identity-token selection (gcloud fallback)
and the fixed-label cloud initialization failure. It did not produce a complete
cloud plan or fingerprint, create an operation journal, or perform cloud/API mutations.

The active gcloud login and successful Platform API reads do **not** supply or prove
the Google Cloud SDK credentials/authority used by the new IAM executor. No login,
credential file, service-account impersonation, IAM grant or service enablement was
silently configured to work around this failure.

## Resume gate

1. The operator must configure the intended ADC identity, normally with
   `gcloud auth application-default login` for interactive user ADC, or an explicitly
   approved existing service-account/WIF setup. This is credential setup, not a
   request to grant IAM Admin/Owner or broaden permissions.
2. Re-run only the checksum-verified candidate's read-only preview. Validate its
   actual cloud identity, required apply/cleanup authority and permission support.
3. Complete the remaining read-only footprint/quota/routing/bootstrap/guest-access
   checks and priced estimate, including management workloads and transient capacity.
   These checks are not claimed complete in this checkpoint.
4. Review the exact resource/cost/retention manifest with the user. The proposed
   USD 100 allowance is still unapproved, not a validated estimate or hard billing cap.
   The old November 19 00:00Z deadline must be explicitly reconfirmed/replaced.
5. Only after separate approval may the bounded cleanup smoke and live activation/
   installation proceed. Establish a deterministic safe smoke stop point first;
   do not race an interrupt or use legacy IAM/network paths.

The [live proposal](phase-3-live-proposal.md) remains **unapproved**. Phase 3 live
acceptance and Phase 4 have not started. Both gates stay false and no customer IAM,
network, Cluster, NodePool or hosted resource was allocated by this preflight.
