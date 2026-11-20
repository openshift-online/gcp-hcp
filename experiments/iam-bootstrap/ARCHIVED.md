# IAM bootstrap POC — concluded

**Concluded:** 2026-11-20. Implementation source is preserved in the dedicated
`poc/wifconfigtemplate` branches, not in this documentation directory:

- [Gecko](https://github.com/apahim/gecko/tree/poc/wifconfigtemplate), reference
  `6c28cf4348a8ab4f524b91ece1cdbe29f1389c44`.
- [gcp-hcp-ctl](https://github.com/apahim/gcp-hcp-ctl/tree/poc/wifconfigtemplate),
  reference `ba211636d48d6cedf11e6090536ef7c5f985800f`.

Implementation proceeds in two increments:

1. [Explicit component permissions, catalog and IAM lifecycle](../../implementation-plans/wifconfigtemplate-catalog-and-iam-lifecycle.md).
2. [Dependent resource-name-prefix conditions and component changes](../../implementation-plans/wifconfigtemplate-resource-name-prefix-conditions.md).

The POC proved an actual 4.22.21 → exact 5.0.0 IAM-gated control-plane/worker
upgrade and owned secondary cleanup. At the final recorded readback on 2026-11-20,
the secondary hosted/VM/IAM/network resources were removed or soft-deleted, both
new-use gates were closed, and the original validation installation and shared
platform were intentionally retained. This is not a fresh health assertion or
permission to destroy the retained environment.

AI session notes, obsolete proposals and retired one-off probes were retained in
a private archive outside the repository. The early prefix prototype was not
retained as a complete source snapshot in the locations checked; Plan 2 records
its research anchors and reconstruction requirements, not a claim of shipped code.
The plans capture durable findings and evidence limits;
neither those archived probes nor the historical approvals are execution guidance.
