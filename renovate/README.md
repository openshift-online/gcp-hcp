# Shared Renovate preset

Opt in from a repository's root `renovate.json`:

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": ["github>openshift-online/gcp-hcp//renovate/default"]
}
```

The preset enables the dependency dashboard, adds `ok-to-test` and
`area/dependency`, limits concurrent PRs to 10, groups `golang.org/x` modules,
and runs Go module tidy and import-path updates. Automerge and dashboard
vulnerability summaries are disabled; vulnerability alerts remain enabled by
MintMaker. It does not onboard this repository to Renovate.

`registry.access.redhat.com/ubi9/go-toolset` updates stay on Go `1.x` tags, not
UBI `9.x` tags. Go 2 requires revisiting this constraint; `ubi-micro` is unaffected.
Go module major updates are disabled and require manual source and dependency
migration. Other major updates require dashboard approval before branch and PR
creation; Renovate's security-update exemptions remain in effect. Go language
minor upgrades and v0 module minor updates remain eligible.

Keep repository-specific settings in the consumer config. Put Go-only overrides
under `gomod`; for connected local modules, use
`"gomod": {"postUpdateOptions": ["gomodTidyAll", "gomodUpdateImportPaths"]}`.
`gomodTidyAll` implies `gomodTidy`. Renovate merges these options with inherited
arrays; use narrowly scoped package rules for deliberate policy exceptions.

Validate from this repository's root against MintMaker's Renovate base version:

```sh
npx --package=renovate@44.209.0 -- renovate-config-validator --no-global --strict renovate/default.json
```

Publish the preset to this repository's default branch before merging consumer
references. Validate each consumer from its root with
`npx --package=renovate@44.209.0 -- renovate-config-validator --no-global --strict renovate.json`.
The validator does not resolve top-level `extends`; also verify preset resolution
and effective configuration in Renovate/MintMaker.
