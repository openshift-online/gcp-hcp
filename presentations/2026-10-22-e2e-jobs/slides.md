---
theme: default
layout: default
title: GCP HCP end-to-end jobs
aspectRatio: 16/9
canvasWidth: 1280
colorSchema: dark
fonts:
  sans: Red Hat Text
  mono: Red Hat Mono
  provider: none
---

<div class="rh-slide cover">
  <div class="brandline"><span><span class="brand-accent"></span>GCP HCP engineering</span><span class="deck-kind">Demo Oct 22 2026</span></div>
  <div class="cover-main">
    <h1>How we test the <span class="cover-accent">GCP HCP platform</span></h1>
    <p class="cover-subtitle">Three complementary signals, one shared HostedCluster lifecycle test.</p>
    <div class="cover-cards">
      <div class="cover-card"><strong>Full platform E2E</strong><span>Temporary stack, complete platform path</span></div>
      <div class="cover-card"><strong>Targeted INT / STAGE</strong><span>Real, long-running environments</span></div>
      <div class="cover-card"><strong>Cross-repo E2E <em class="wip-tag small">In progress</em></strong><span>Candidate component images</span></div>
    </div>
  </div>
  <div class="deck-footer"><span>Choose the signal that matches the change you are testing.</span><a href="https://redhat.atlassian.net/wiki/spaces/GCP/pages/506888659/GCP+HCP+E2E+Jobs+Team+Cheat+Sheet" target="_blank">Team cheat sheet ↗</a><span>01 / 05</span></div>
</div>

<!--
Open with the job selection question: do we need the whole platform, the deployed environment, or an unmerged component candidate?
The full infra PR job and targeted jobs both exercise the HostedCluster Ginkgo v2 lifecycle; they differ in what they provision and which code/environment they test.
Source: [final team cheat sheet](https://redhat.atlassian.net/wiki/spaces/GCP/pages/506888659/GCP+HCP+E2E+Jobs+Team+Cheat+Sheet).
-->

---

<div class="rh-slide">
  <div class="brandline"><span><span class="brand-accent"></span>GCP HCP engineering</span><span class="deck-kind">Full platform E2E</span></div>
  <div class="slide-heading">
    <p class="slide-kicker">Full platform E2E</p>
    <h2>Build it, test it, gather, tear it down</h2>
    <p class="slide-subtitle">Run <code>/test e2e-platform</code> on an infra PR, or let the weekday <code>main</code> periodic exercise the baseline.</p>
  </div>
  <div class="flow">
    <div class="flow-node"><span class="flow-step">1 · Trigger</span><strong>PR test or cron</strong><small>Pending status on tested SHA</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">2 · Identity</span><strong>WIF credentials</strong><small>Access the test stack</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">3 · Provision</span><strong>Terraform Cloud</strong><small>Ephemeral region + customer</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">4 · Verify</span><strong>Ginkgo v2</strong><small>ArgoCD + HC lifecycle</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">5 · Diagnose</span><strong>Gather</strong><small>Artifacts before teardown</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">6 · Finish</span><strong>Destroy stack</strong><small>Prow result + status</small></div>
  </div>
  <div class="callouts">
    <div class="callout"><strong>Terraform Cloud</strong><code>provision.log</code> links to the run-specific workspace and plan/apply output.</div>
    <div class="callout"><strong>Prow / Spyglass</strong>Explore step logs, Ginkgo results, gather artifacts, and teardown output in the run.</div>
  </div>
  <div class="deck-footer"><span>Weekday periodic: 02:00 / 14:00 UTC · Region: <code>northamerica-northeast2</code></span><a href="https://github.com/openshift/release/blob/main/ci-operator/step-registry/gcp-hcp/e2e/gcp-hcp-e2e-workflow.yaml" target="_blank">Workflow source ↗</a><span>02 / 05</span></div>
</div>

<!--
Show the exact PR command: `/test e2e-platform` on an openshift-online/gcp-hcp-infra PR.
The job creates an ephemeral region/management cluster/customer project through Terraform Cloud. The provision step prints the remote workspace URL in provision.log.
Ginkgo v2 uses the argocd and hc-lifecycle labels; the HC test calls gcphcpctl through the same public API path a customer uses.
Gather runs before teardown and writes a bounded, redacted gather/ bundle, summary, and manifest under Prow artifacts. Cleanup deletes the test projects and clears Terraform workspace state.
The e2e/platform GitHub status records the tested infra SHA. Prow / Spyglass is the complete record for step logs, Ginkgo results, gather artifacts, and teardown output.
Sources: [ci-operator config](https://github.com/openshift/release/blob/main/ci-operator/config/openshift-online/gcp-hcp-infra/openshift-online-gcp-hcp-infra-main.yaml), [workflow](https://github.com/openshift/release/blob/main/ci-operator/step-registry/gcp-hcp/e2e/gcp-hcp-e2e-workflow.yaml), [Ginkgo suites](https://github.com/openshift-online/gcp-hcp-infra/blob/main/test/e2e/README.md), [gather step](https://github.com/openshift/release/blob/main/ci-operator/step-registry/gcp-hcp/gather/gcp-hcp-gather-commands.sh).
-->

---

<div class="rh-slide">
  <div class="brandline"><span><span class="brand-accent"></span>GCP HCP engineering</span><span class="deck-kind">Targeted HostedCluster</span></div>
  <div class="slide-heading">
    <p class="slide-kicker">Targeted INT / STAGE</p>
    <h2>Test the customer path in INT and STAGE</h2>
    <p class="slide-subtitle">Same Ginkgo lifecycle, existing customer projects, no platform provisioning.</p>
  </div>
  <div class="flow compact" style="margin-top: 22px">
    <div class="flow-node"><span class="flow-step">1 · Trigger</span><strong>Merge or cron</strong><small>Sector branch / main</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">2 · Postsubmit</span><strong>Wait for deploy</strong><small>Pending status, then Terraform + ArgoCD</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">3 · Customer</span><strong>Public API</strong><small>WIF + existing project</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">4 · Lifecycle</span><strong>Full HC lifecycle</strong><small>Create → workload → delete</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">5 · Verify</span><strong>Cleanup + status</strong><small>Final status on sector SHA</small></div>
  </div>
  <div class="small-banner gate"><strong>Gate still planned:</strong> Promoter does not yet consume the INT <code>e2e/hostedcluster-int</code> result. Check INT before promoting to STAGE.</div>
  <div class="env-grid compact">
    <div class="env-card"><h3>INT · us-central1</h3><div class="env-line"><span>Customer project</span><span>int-ci-cust-us-c1</span></div><div class="env-line"><span>Postsubmit</span><span>sector-integration-main</span></div><div class="env-line"><span>Main periodic</span><span>03:00 / 15:00 UTC</span></div></div>
    <div class="env-card"><h3>STAGE · us-west1</h3><div class="env-line"><span>Customer project</span><span>stg-ci-cust-us-w1</span></div><div class="env-line"><span>Postsubmit</span><span>sector-stage-main</span></div><div class="env-line"><span>Main periodic</span><span>04:00 / 16:00 UTC</span></div></div>
  </div>
  <div class="deck-footer"><span>Periodics check health; postsubmits publish status on the tested sector SHA.</span><a href="https://github.com/openshift/release/blob/main/ci-operator/step-registry/gcp-hcp/e2e-on-demand/gcp-hcp-e2e-on-demand-workflow.yaml" target="_blank">Workflow source ↗</a><span>03 / 05</span></div>
</div>

<!--
INT postsubmit runs only after a merge to environment/sector-integration-main; STAGE postsubmit runs only after a merge to environment/sector-stage-main. Main periodics run at the times on the slide, weekdays.
The postsubmit captures its active sector SHA, posts a pending e2e/hostedcluster-* status on it, waits up to 30 minutes for Terraform apply and ArgoCD health checks for that SHA, then starts the lifecycle. The finalizer requires lifecycle and cleanup success and checks the branch still points to the captured SHA before posting success/failure. Periodics and PR presubmits do not publish the deployed-commit status.
The cleanup post-step verifies nodepool, cluster, worker VMs, network, and IAM resources are absent. The persistent customer project remains.
Current job config uses candidate 5.0.0-ec.6 in INT (test default) and pins stable 4.22.14 in STAGE pending 5.x validation.
The sector promotion strategy still does not consume e2e/hostedcluster-int as an INT-to-STAGE gate. Until it does, humans should verify INT postsubmit health before STAGE promotion/merge. A postsubmit cannot block the INT merge that triggered it.
Sources: [on-demand workflow](https://github.com/openshift/release/blob/main/ci-operator/step-registry/gcp-hcp/e2e-on-demand/gcp-hcp-e2e-on-demand-workflow.yaml), [status updater](https://github.com/openshift/release/blob/main/ci-operator/step-registry/gcp-hcp/update-environment-status/gcp-hcp-update-environment-status-commands.sh), [finalizer](https://github.com/openshift/release/blob/main/ci-operator/step-registry/gcp-hcp/finalize-environment-status/gcp-hcp-finalize-environment-status-commands.sh), [sector promotion strategy](https://github.com/openshift-online/gcp-hcp-infra/blob/main/helm/charts/gitops-promoter-config-region/templates/promotion-strategy.yaml).
-->

---

<div class="rh-slide">
  <div class="brandline"><span><span class="brand-accent"></span>GCP HCP engineering</span><span class="deck-kind">Signals and triage</span></div>
  <div class="slide-heading">
    <p class="slide-kicker">Read the right signal</p>
    <h2>Find the failure, then decide who acts</h2>
    <p class="slide-subtitle">Prow shows each run, GitHub ties results to commits, and Sippy reveals trends.</p>
  </div>
  <div class="signal-grid">
    <div class="signal-card"><h3>Prow + artifacts</h3><p>The authoritative result. Start at the first failed step or JUnit spec.</p><p class="muted">Full: gather summary, then TFC plan/apply. Targeted: deploy wait, lifecycle, or cleanup.</p></div>
    <div class="signal-card"><h3>GitHub status</h3><p><code>e2e/platform</code> and <code>e2e/hostedcluster-*</code> mark the tested SHA.</p><p class="muted">Compare the postsubmit SHA with the active sector branch.</p></div>
    <div class="signal-card"><h3>Sippy + Chai Bot</h3><p>Sippy tracks full-periodic Ginkgo results and alerts on failures.</p><p class="muted">Chai Bot picks up the alert and posts a Slack diagnosis, with matching failure signatures from other runs.</p></div>
  </div>
  <div class="who-acts"><strong>Who acts</strong><span><b>Infra PR red</b> → PR author fixes</span><span><b>INT postsubmit red</b> → hold STAGE</span><span><b>Periodic red</b> → e2e-watcher triages</span></div>
  <div class="live-strip"><strong>Useful links</strong><div class="live-links"><a href="https://prow.ci.openshift.org/job-history/gs/test-platform-results-public/logs/periodic-ci-openshift-online-gcp-hcp-infra-main-e2e-platform" target="_blank">Full E2E ↗</a><a href="https://prow.ci.openshift.org/job-history/gs/test-platform-results-public/logs/periodic-ci-openshift-online-gcp-hcp-infra-main-e2e-hostedcluster-int" target="_blank">INT ↗</a><a href="https://prow.ci.openshift.org/job-history/gs/test-platform-results-public/logs/periodic-ci-openshift-online-gcp-hcp-infra-main-e2e-hostedcluster-stg" target="_blank">STAGE ↗</a><a href="https://sippy.dptools.openshift.org/sippy-ng/jobs/gcp-hcp-main" target="_blank">Sippy ↗</a><a href="https://prow.ci.openshift.org/view/gs/test-platform-results-public/pr-logs/pull/openshift-online_gcp-hcp-infra/2407/pull-ci-openshift-online-gcp-hcp-infra-main-e2e-hostedcluster-stg/2112280642090700800" target="_blank">Example run ↗</a><a href="https://github.com/openshift-online/gcp-hcp-infra/pull/2407" target="_blank">Example PR ↗</a><a href="https://github.com/openshift-online/gcp-hcp-infra/commits/environment/sector-integration-main/" target="_blank">INT commits ↗</a></div></div>
  <div class="deck-footer"><span>Use the tested SHA and Prow result before drawing a promotion conclusion.</span><a href="https://github.com/openshift/sippy/pull/4029" target="_blank">Sippy onboarding ↗</a><span>04 / 05</span></div>
</div>

<!--
The live links include the current job histories and Sippy trends. "Example run" is a completed STAGE Spyglass run; "Example PR" shows its status checks on infra PR #2407. "INT commits" is the environment/sector-integration-main branch history; open a commit's status checks to show Terraform, ArgoCD, and e2e/hostedcluster-int on the promoted SHA. Use current histories for present-day health.
For full E2E, the gather step runs before teardown and provides a bounded/redacted bundle. The Terraform Cloud workspace URL is in provision.log.
For targeted jobs, separate failures in deployment-check wait, API/lifecycle, and cleanup verification. On postsubmits, compare the tested SHA with the current active branch SHA.
Sippy onboarding PR #4029 mapped the full periodic to synthetic gcp-hcp-main and enabled Ginkgo JUnit import. Sippy visibility and PagerDuty alert routing need confirmation; do not imply automatic paging. Targeted-job Sippy mapping PR #4106 is draft in the source cheat sheet.
Chai Bot is our internal Slack bot: when Sippy alerts, it investigates the failure and posts a summary that includes trends (failure signatures) across other failed runs. The e2e-watcher is the human follow-up rotation that acts on that summary: [schedule](https://gitlab.cee.redhat.com/service/app-interface/-/blob/master/data/teams/gcp-hcp/schedules/gcp-hcp-e2e-watcher.yml).
Sources: [full periodic history](https://prow.ci.openshift.org/job-history/gs/test-platform-results-public/logs/periodic-ci-openshift-online-gcp-hcp-infra-main-e2e-platform), [INT periodic history](https://prow.ci.openshift.org/job-history/gs/test-platform-results-public/logs/periodic-ci-openshift-online-gcp-hcp-infra-main-e2e-hostedcluster-int), [STAGE periodic history](https://prow.ci.openshift.org/job-history/gs/test-platform-results-public/logs/periodic-ci-openshift-online-gcp-hcp-infra-main-e2e-hostedcluster-stg).
-->

---

<div class="rh-slide">
  <div class="brandline"><span><span class="brand-accent"></span>GCP HCP engineering</span><span class="deck-kind">Cross-repo E2E</span></div>
  <div class="slide-heading">
    <p class="slide-kicker">Next workstream</p>
    <h2>Test component candidates in the full stack <span class="wip-tag">In progress</span></h2>
    <p class="slide-subtitle">Reuse the trusted full E2E harness, but deploy immutable candidate artifacts from other repositories.</p>
  </div>
  <div class="flow compact" style="margin-top: 25px">
    <div class="flow-node"><span class="flow-step">1 · Candidate</span><strong>Component PR</strong><small>Request a cross-repo test</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">2 · Artifact</span><strong>Konflux snapshot</strong><small>Image pinned by digest</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">3 · Infra</span><strong>Trusted baseline</strong><small>Terraform provisioning</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">4 · Exercise</span><strong>Candidate deploy</strong><small>Shared Ginkgo lifecycle</small></div><div class="flow-arrow">→</div>
    <div class="flow-node"><span class="flow-step">5 · Finish</span><strong>Gather + teardown</strong><small>PR test result</small></div>
  </div>
  <div class="candidate-grid compact">
    <div class="candidate-card"><h3>Candidate images</h3><strong>Gecko · kube-applier-gcp · HyperShift</strong><p class="muted">The candidate image and source revision must match the PR. The HyperShift handoff is still being finalized.</p></div>
    <div class="candidate-card"><h3>CLI design choice</h3><strong>Runner image or build from PR SHA</strong><p class="muted">An image route also needs Konflux onboarding and a release pipeline in the gcp-hcp-ctl repo.</p></div>
  </div>
  <div class="deck-footer"><span>Trusted infra provisions Terraform; component PR code cannot. Proposed test: <code>/test gcp-e2e-platform</code>.</span><a href="https://github.com/openshift-online/gcp-hcp-infra" target="_blank">Infra harness ↗</a><span>05 / 05</span></div>
</div>

<!--
This flow is a design in progress, not a currently shipped Prow job.
Gecko candidate covers the Platform API/controllers; kube-applier-gcp candidate covers management-cluster application; HyperShift candidate covers operator/control-plane pieces. The goal is immutable Konflux PR snapshot images pinned by digest and matched to the PR source revision.
The CLI choice is open: use a candidate CLI runner image based on a container, or build gcphcpctl from the PR SHA per run. The image route still needs Konflux onboarding and some release pipeline.
Keep Terraform provisioning on the trusted gcp-hcp-infra baseline, then reuse the full E2E Ginkgo suites, gather, and teardown. Do not run arbitrary component PR code as Terraform provisioning.
Sources: [Gecko](https://github.com/openshift-online/gecko), [kube-applier-gcp](https://github.com/openshift-online/kube-applier-gcp), [HyperShift](https://github.com/openshift/hypershift), [gcp-hcp-ctl](https://github.com/openshift-online/gcp-hcp-ctl), [full E2E workflow](https://github.com/openshift/release/blob/main/ci-operator/step-registry/gcp-hcp/e2e/gcp-hcp-e2e-workflow.yaml).
-->
