#!/usr/bin/env bash
# Offline checks for the GCP-1280 Step 2 chart packaging and the inactive
# MC/region bundles. Uses real helm, kubectl kustomize, yq and jq against the
# committed archives and bundles. Contacts no network and no cluster.
#
# Run from anywhere: tools/gitops-bootstrap/tests/run-step2-tests.sh
# Optional: GITOPS_BUNDLE_DIR=<verified extracted official bundle dir> adds a
# byte-level check of the vendored CRDs against the official bundle.
#
# BOOTSTRAP_BASE_REF (default: origin/main) selects the PR base the active
# bootstrap/ must be unchanged against. The suite compares bootstrap/ with the
# merge base of that ref and HEAD, so committed changes fail even when the
# working tree is clean, and index, worktree and untracked changes fail too.
# The ref (a branch, remote-tracking ref or commit SHA) must already exist
# locally and share history with HEAD; the suite never fetches, never falls
# back to HEAD and never guesses another remote, and it aborts before any other
# test if the base cannot be resolved. In a fork checkout whose origin/main is
# not the PR base, pass the right local ref, e.g.:
#   BOOTSTRAP_BASE_REF=upstream/main tools/gitops-bootstrap/tests/run-step2-tests.sh
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOL="$(dirname "$HERE")"
ROOT="$(cd "$TOOL/../.." && pwd)"
T="$(mktemp -d "${TMPDIR:-/tmp}/gcp-1280-step2-tests.XXXXXX")"
trap 'rm -rf "$T"' EXIT

PASS=0
FAIL=0
pass() { PASS=$((PASS + 1)); echo "ok   - $1"; }
fail() { FAIL=$((FAIL + 1)); echo "FAIL - $1"; }
check() { # check <description> <command...>
  local d="$1"; shift
  if "$@" >/dev/null 2>&1; then pass "$d"; else fail "$d"; fi
}
expect_fail() { # expect_fail <description> <stderr-pattern> <command...>
  local d="$1" pat="$2"; shift 2
  local out
  if out="$("$@" 2>&1)"; then
    fail "$d (unexpectedly succeeded)"
  elif grep -qE -- "$pat" <<<"$out"; then
    pass "$d"
  else
    fail "$d (wrong error: $(head -c 200 <<<"$out"))"
  fi
}

# --- active bootstrap/ guard (helpers are reused by the synthetic fixtures below)
BOOTSTRAP_BASE_REF="${BOOTSTRAP_BASE_REF:-origin/main}"

# resolve_bootstrap_base <repo> <ref>: print the merge base of <ref> and HEAD.
resolve_bootstrap_base() {
  local repo="$1" ref="$2" commit base
  if ! commit="$(git -C "$repo" rev-parse --verify --quiet --end-of-options "${ref}^{commit}")"; then
    echo "BOOTSTRAP_BASE_REF '$ref' does not resolve to a commit in this repository; the suite never fetches, so supply a locally available PR base (branch, remote-tracking ref or commit SHA)" >&2
    return 1
  fi
  if ! base="$(git -C "$repo" merge-base "$commit" HEAD)"; then
    echo "BOOTSTRAP_BASE_REF '$ref' shares no history (no merge base) with HEAD; supply the PR base with shared history" >&2
    return 1
  fi
  printf '%s\n' "$base"
}

# bootstrap_unchanged <repo> <merge-base>: bootstrap/ equals the merge base,
# including committed, staged and unstaged edits, and has no untracked files.
bootstrap_unchanged() {
  local repo="$1" base="$2"
  if ! git -C "$repo" diff --quiet "$base" -- bootstrap; then
    echo "bootstrap/ differs from merge base $base (committed, staged or unstaged change)" >&2
    return 1
  fi
  if [[ -n "$(git -C "$repo" status --porcelain -- bootstrap)" ]]; then
    echo "bootstrap/ has staged, unstaged or untracked changes" >&2
    return 1
  fi
}

# Fail early, before any expensive test, if the comparison base is unusable.
if ! BOOTSTRAP_BASE_COMMIT="$(resolve_bootstrap_base "$ROOT" "$BOOTSTRAP_BASE_REF")"; then
  echo "run-step2-tests.sh: cannot select the active-bootstrap comparison base; aborting before any test" >&2
  exit 1
fi
echo "bootstrap comparison base: $BOOTSTRAP_BASE_REF -> merge base $BOOTSTRAP_BASE_COMMIT"

PINS="$TOOL/pins.yaml"
ALLOW="$TOOL/allowed-images.txt"
CSV="$TOOL/chart-src/upstream/gitops-operator.v1.22.2.clusterserviceversion.yaml"
SYN_PROJECT="synthetic-global-project"
sha() { shasum -a 256 "$1" | awk '{print $1}'; }
j() { yq eval -o=json -I=0 '.' "$1" | jq -s '.'; }

# ---------------------------------------------------------------- archives
for c in crdsChart operatorChart; do
  path="$(yq eval ".$c.path" "$PINS")"; want="$(yq eval ".$c.checksum" "$PINS")"
  check "$c archive SHA256 equals its pin" test "sha256:$(sha "$TOOL/$path")" = "$want"
done
mkdir "$T/x"
tar -xzf "$TOOL/charts/gitops-operator-crds-0.1.0.tgz" -C "$T/x"
tar -xzf "$TOOL/charts/gitops-operator-0.1.0.tgz" -C "$T/x"
check "both charts carry the verified Apache-2.0 LICENSE (upstream commit cf327714…)" \
  bash -c "[[ \$(shasum -a 256 '$T/x/gitops-operator/LICENSE' | cut -d' ' -f1) == 43070e2d4e532684de521b885f385d0841030efa2b1a20bafb76133a5e1379c1 && \$(shasum -a 256 '$T/x/gitops-operator-crds/LICENSE' | cut -d' ' -f1) == 43070e2d4e532684de521b885f385d0841030efa2b1a20bafb76133a5e1379c1 ]]"
check "both packaged NOTICE files carry the changed-file notice for the adapted ArgoCD CRD and match chart-src/NOTICE" \
  bash -c "cmp '$T/x/gitops-operator/NOTICE' '$TOOL/chart-src/NOTICE' && cmp '$T/x/gitops-operator-crds/NOTICE' '$TOOL/chart-src/NOTICE' && grep -q 'MODIFIED copy' '$TOOL/chart-src/NOTICE' && grep -q 'v1alpha1 entry' '$TOOL/chart-src/NOTICE' && grep -q 'spec.conversion' '$TOOL/chart-src/NOTICE'"
check "chart versions are local packaging 0.1.0 with official appVersion 1.22.2" \
  bash -c "[[ \$(yq eval '.version + \"/\" + .appVersion' '$T/x/gitops-operator/Chart.yaml') == '0.1.0/1.22.2' && \$(yq eval '.version + \"/\" + .appVersion' '$T/x/gitops-operator-crds/Chart.yaml') == '0.1.0/1.22.2' ]]"
check "pins record the official bundle digest, not an invented archive digest" \
  bash -c "[[ \$(yq eval '.crdsChart.digest' '$PINS') == sha256:2ba58c50f771c08bbc5a1d8572c7b058df55bc71667cc66026d42840eb4bee6d && \$(yq eval '.operatorChart.digest' '$PINS') == sha256:2ba58c50f771c08bbc5a1d8572c7b058df55bc71667cc66026d42840eb4bee6d ]]"

crd_names="$(ls "$T/x/gitops-operator-crds/crds" | LC_ALL=C sort | tr '\n' ' ')"
check "CRD archive holds exactly the 8 selected CRDs (no ImageUpdater/Rollouts/Analysis/Experiments/GitopsService)" \
  test "$crd_names" = "applications.argoproj.io.yaml applicationsets.argoproj.io.yaml appprojects.argoproj.io.yaml argocds.argoproj.io.yaml controllerconfigurations.promoter.argoproj.io.yaml namespacemanagements.argoproj.io.yaml notificationsconfigurations.argoproj.io.yaml rolloutmanagers.argoproj.io.yaml "
ok=1
while read -r sum rel; do
  [[ "$(sha "$T/x/gitops-operator-crds/$rel")" == "$sum" ]] || ok=0
done < "$TOOL/chart-src/vendored-crds.sha256"
check "archive CRDs match the recorded vendored digests" test "$ok" = 1
ok=1
for pair in applications.argoproj.io:argoproj.io_applications applicationsets.argoproj.io:argoproj.io_applicationsets \
  appprojects.argoproj.io:argoproj.io_appprojects rolloutmanagers.argoproj.io:argoproj.io_rolloutmanagers \
  namespacemanagements.argoproj.io:argoproj.io_namespacemanagements notificationsconfigurations.argoproj.io:argoproj.io_notificationsconfigurations \
  controllerconfigurations.promoter.argoproj.io:promoter.argoproj.io_controllerconfigurations; do
  up="$(grep " manifests/${pair#*:}.yaml\$" "$TOOL/chart-src/upstream.sha256" | cut -d' ' -f1)"
  [[ -n "$up" && "$(sha "$T/x/gitops-operator-crds/crds/${pair%%:*}.yaml")" == "$up" ]] || ok=0
done
check "7 of 8 CRDs are byte-identical to the official bundle files" test "$ok" = 1
argocds="$T/x/gitops-operator-crds/crds/argocds.argoproj.io.yaml"
check "ArgoCD CRD adaptation: v1beta1 only (storage), no conversion" \
  bash -c "yq eval -o=json -I=0 '.' '$argocds' | jq -e '(.spec.versions | map(.name)) == [\"v1beta1\"] and .spec.versions[0].storage == true and .spec.versions[0].served == true and (.spec.conversion == null)'"
if [[ -n "${GITOPS_BUNDLE_DIR:-}" && -f "$GITOPS_BUNDLE_DIR/manifests/argoproj.io_argocds.yaml" ]]; then
  check "ArgoCD v1beta1 block equals the official bundle's byte-for-byte after canonicalization" \
    bash -c "cmp <(yq eval -o=json '.spec.versions[] | select(.name==\"v1beta1\")' '$GITOPS_BUNDLE_DIR/manifests/argoproj.io_argocds.yaml' | jq -S .) <(yq eval -o=json '.spec.versions[0]' '$argocds' | jq -S .)"
fi

# --------------------------------------------------- render + CSV parity
RENDER_ARGS=(--pins "$PINS" --namespace gitops-operator --instance-namespace argocd
  --service-account openshift-gitops-operator-controller-manager --image-pull-secret-name redhat-registry-pull-secret
  --image-allowlist "$ALLOW")
if "$TOOL/render.sh" "${RENDER_ARGS[@]}" --output-dir "$T/render" >/dev/null 2>&1; then
  pass "render.sh renders the real pinned charts under all existing guardrails"
else
  fail "render.sh renders the real pinned charts under all existing guardrails"
fi
yq eval -o=json -I=0 '.' "$CSV" > "$T/csv.json"
j "$T/render/operator.yaml" > "$T/op.json"
jq -c '.spec.install.spec.deployments[0].spec.template.spec.containers[] | select(.name=="manager")' "$T/csv.json" > "$T/csv-mgr.json"
jq -c '.[] | select(.kind=="Deployment") | .spec.template.spec.containers[] | select(.name=="manager")' "$T/op.json" > "$T/op-mgr.json"
ENVMAP='(.env | map({(.name): (.value // "")}) | add)'
check "manager command/args/probes/ports/securityContext/mounts/image equal the official CSV (resources exempted below)" \
  bash -c "diff <(jq -S '{command,args,livenessProbe,readinessProbe,ports,securityContext,volumeMounts,image}' '$T/csv-mgr.json') <(jq -S '{command,args,livenessProbe,readinessProbe,ports,securityContext,volumeMounts,image}' '$T/op-mgr.json')"
check "the only manager resources delta: CSV sets none, the packaging sets exactly the documented untuned initial budget" \
  bash -c "jq -e '.resources == {}' '$T/csv-mgr.json' && jq -e '.resources == {requests:{cpu:\"100m\",memory:\"256Mi\"}, limits:{cpu:\"1\",memory:\"1Gi\"}}' '$T/op-mgr.json'"
check "manager env equals the official CSV env except the three documented GKE deltas" \
  bash -c "diff <(jq -S '$ENVMAP | .ENABLE_CONVERSION_WEBHOOK=\"false\" | .ARGOCD_CLUSTER_CONFIG_NAMESPACES=\"argocd\" | .WATCH_NAMESPACE=\"\"' '$T/csv-mgr.json') <(jq -S '$ENVMAP' '$T/op-mgr.json')"
check "no manager image env is missing (31 image env names, none extra)" \
  bash -c "diff <(jq -r '.env[].name | select(test(\"IMAGE\"))' '$T/csv-mgr.json' | sort) <(jq -r '.env[].name | select(test(\"IMAGE\"))' '$T/op-mgr.json' | sort) && [[ \$(jq '[.env[].name | select(test(\"IMAGE\"))] | length' '$T/op-mgr.json') == 31 ]]"
check "pod spec parity: nonroot, grace period, metrics-certs volume, service account, pull secret" \
  bash -c "jq -e --slurpfile c '$T/csv.json' '
    (.[] | select(.kind==\"Deployment\") | .spec.template.spec) as \$p
    | (\$c[0].spec.install.spec.deployments[0].spec.template.spec) as \$o
    | \$p.securityContext == \$o.securityContext and \$p.terminationGracePeriodSeconds == \$o.terminationGracePeriodSeconds
      and \$p.volumes == \$o.volumes and \$p.serviceAccountName == \$o.serviceAccountName
      and (\$p.imagePullSecrets | map(.name)) == [\"redhat-registry-pull-secret\"]' '$T/op.json'"
NORM='[.[] | {apiGroups:(.apiGroups//[]|sort),resources:(.resources//[]|sort),resourceNames:(.resourceNames//[]|sort),nonResourceURLs:(.nonResourceURLs//[]|sort),verbs:(.verbs|sort)}] | sort_by(tostring)'
check "ClusterRole rules equal the official CSV clusterPermissions" \
  bash -c "diff <(jq -S '.spec.install.spec.clusterPermissions[0].rules | $NORM' '$T/csv.json') <(jq -S '.[] | select(.kind==\"ClusterRole\") | .rules | $NORM' '$T/op.json')"
check "leader-election Role rules equal the official CSV permissions" \
  bash -c "diff <(jq -S '.spec.install.spec.permissions[0].rules | $NORM' '$T/csv.json') <(jq -S '.[] | select(.kind==\"Role\") | .rules | $NORM' '$T/op.json')"
check "allowlist equals the CSV's distinct relatedImages (15) and every manager image/env is in it" \
  bash -c "diff <(grep -v '^#' '$ALLOW' | sort) <(jq -r '.spec.relatedImages[].image' '$T/csv.json' | sort -u) && [[ \$(grep -vc '^#' '$ALLOW') == 15 ]] && jq -e --slurpfile a <(grep -v '^#' '$ALLOW' | jq -R .) '[.image, (.env[] | select(.name|test(\"IMAGE\")) | .value)] | all(. as \$v | \$a | index(\$v) != null)' '$T/op-mgr.json'"
check "no floating/unreviewed image survives: every manager image is digest-pinned on registry.redhat.io" \
  bash -c "jq -e '[.image, (.env[] | select(.name|test(\"IMAGE\")) | .value)] | all(test(\"^registry\\\\.redhat\\\\.io/[a-z0-9./_-]+@sha256:[0-9a-f]{64}\$\"))' '$T/op-mgr.json'"
check "status removal is top-level only: nested status schema survives in the rendered CRD" \
  bash -c "j() { yq eval -o=json -I=0 '.' \"\$1\" | jq -s '.'; }; j '$T/render/crds.yaml' | jq -e 'all(.[]; has(\"status\") | not) and (.[] | select(.metadata.name==\"argocds.argoproj.io\") | .spec.versions[0].schema.openAPIV3Schema.properties.status != null)'"

# ------------------------------------------------ render negative cases
grep -v 'argocd-rhel9@' "$ALLOW" > "$T/allow-missing-argocd.txt"
expect_fail "render rejects a tuple missing a reviewed operand image" "unreviewed image tuple" \
  "$TOOL/render.sh" --pins "$PINS" --namespace gitops-operator --instance-namespace argocd \
    --service-account openshift-gitops-operator-controller-manager --image-pull-secret-name redhat-registry-pull-secret \
    --image-allowlist "$T/allow-missing-argocd.txt" --output-dir "$T/neg1"
yq eval ".crdsChart.checksum = \"sha256:$(printf '0%.0s' {1..64})\"" "$PINS" > "$T/pins-bad.yaml"
cp -R "$TOOL/charts" "$T/charts"
expect_fail "render rejects a pin whose checksum does not match the archive" "checksum mismatch" \
  "$TOOL/render.sh" --pins "$T/pins-bad.yaml" --namespace gitops-operator --instance-namespace argocd \
    --service-account openshift-gitops-operator-controller-manager --image-pull-secret-name redhat-registry-pull-secret \
    --image-allowlist "$ALLOW" --output-dir "$T/neg2"
# Packaged-chart guard: only the literal "false" may reach the manager env.
for bad in true TRUE True 1 0 FALSE False "" no; do
  expect_fail "packaged chart rejects ENABLE_CONVERSION_WEBHOOK='$bad' (only \"false\" is allowed)" 'ENABLE_CONVERSION_WEBHOOK must be "false"' \
    helm template x "$T/x/gitops-operator" --namespace gitops-operator --set-string "env.ENABLE_CONVERSION_WEBHOOK=$bad"
done
expect_fail "packaged chart rejects a native Helm boolean true" 'ENABLE_CONVERSION_WEBHOOK must be "false"' \
  helm template x "$T/x/gitops-operator" --namespace gitops-operator --set env.ENABLE_CONVERSION_WEBHOOK=true
expect_fail "packaged chart rejects a removed (null) setting" 'ENABLE_CONVERSION_WEBHOOK must be "false"' \
  helm template x "$T/x/gitops-operator" --namespace gitops-operator --set env.ENABLE_CONVERSION_WEBHOOK=null
check "packaged chart renders the literal \"false\" by default, for string \"false\" and for a native boolean false" \
  bash -c "for a in '' '--set-string env.ENABLE_CONVERSION_WEBHOOK=false' '--set env.ENABLE_CONVERSION_WEBHOOK=false'; do helm template x '$T/x/gitops-operator' --namespace gitops-operator \$a | yq eval -o=json -I=0 'select(.kind==\"Deployment\") | .spec.template.spec.containers[0].env[] | select(.name==\"ENABLE_CONVERSION_WEBHOOK\") | .value' - | grep -qx '\"false\"' || exit 1; done"
expect_fail "operator chart refuses env keys that overlap imageEnv" "also an imageEnv key" \
  helm template x "$T/x/gitops-operator" --namespace gitops-operator --set-string env.ARGOCD_IMAGE=registry.redhat.io/a/b@sha256:$(printf '0%.0s' {1..64})

# The step-1 renderer only inspects *_IMAGE / RELATED_IMAGE_* env names, so a
# changed GITOPS_CONSOLE_PLUGIN_IMAGE_PF5 would pass render.sh; assemble.sh
# must catch it. Build a tampered copy of the real operator chart to prove it.
BAD_REF="registry.redhat.io/openshift-gitops-1/console-plugin-4-18-rhel9@sha256:$(printf 'a%.0s' {1..64})"
mkdir -p "$T/tamper/gitops-operator" "$T/tamper-pkg"
cp -R "$T/x/gitops-operator/." "$T/tamper/gitops-operator/"
sed "s|^  GITOPS_CONSOLE_PLUGIN_IMAGE_PF5: .*|  GITOPS_CONSOLE_PLUGIN_IMAGE_PF5: $BAD_REF|" "$T/x/gitops-operator/values.yaml" > "$T/tamper/gitops-operator/values.yaml"
python3 -I "$TOOL/chart-src/package-chart.py" "$T/tamper/gitops-operator" "$T/tamper-pkg" >/dev/null 2>&1
yq eval ".operatorChart.path = \"$T/tamper-pkg/gitops-operator-0.1.0.tgz\" | .operatorChart.checksum = \"sha256:$(sha "$T/tamper-pkg/gitops-operator-0.1.0.tgz")\" | .crdsChart.path = \"$TOOL/charts/gitops-operator-crds-0.1.0.tgz\"" "$PINS" > "$T/pins-tamper.yaml"
check "tampered chart passes step-1 render.sh alone (documents the *_IMAGE_PF5 pattern gap)" \
  "$TOOL/render.sh" --pins "$T/pins-tamper.yaml" --namespace gitops-operator --instance-namespace argocd \
    --service-account openshift-gitops-operator-controller-manager --image-pull-secret-name redhat-registry-pull-secret \
    --image-allowlist "$ALLOW" --output-dir "$T/tamper-render"
expect_fail "assemble.sh rejects the unreviewed GITOPS_CONSOLE_PLUGIN_IMAGE_PF5 value render.sh cannot see" "not in the reviewed allowlist" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --pins "$T/pins-tamper.yaml" --output-dir "$T/tamper-out"

# ------------------------------------------------------------- assembly
check "assemble mc" "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --output-dir "$T/mc" --shared-crds-output "$T/mc-shared.yaml"
check "assemble region" "$TOOL/assemble.sh" --type region --global-project-id "$SYN_PROJECT" --output-dir "$T/region" --shared-crds-output "$T/region-shared.yaml"
check "assembly is deterministic (mc twice, byte-identical)" bash -c "'$TOOL/assemble.sh' --type mc --global-project-id $SYN_PROJECT --output-dir '$T/mc2' >/dev/null && diff -r '$T/mc' '$T/mc2'"
check "mc and region differ only by the target-type label" \
  bash -c "sed 's/target-type: mc/target-type: region/' '$T/mc/registry.yaml' | diff - '$T/region/registry.yaml' && sed 's/target-type: mc/target-type: region/' '$T/mc/networkpolicy.yaml' | diff - '$T/region/networkpolicy.yaml' && sed 's/target-type: mc/target-type: region/' '$T/mc/gitops-instance.yaml' | diff - '$T/region/gitops-instance.yaml' && sed 's/target-type: mc/target-type: region/' '$T/mc/controller-rbac.yaml' | diff - '$T/region/controller-rbac.yaml' && cmp '$T/mc/crds.yaml' '$T/region/crds.yaml' && cmp '$T/mc/operator.yaml' '$T/region/operator.yaml' && cmp '$T/mc/kustomization.yaml' '$T/region/kustomization.yaml'"
check "shared CRD artifact identical for mc and region" cmp "$T/mc-shared.yaml" "$T/region-shared.yaml"

for bad in global Global GLOBAL "" mc2 regions " mc"; do
  rm -rf "$T/rej"
  expect_fail "type '${bad:-<empty>}' is rejected at entry; nothing is created" "unsupported --type|missing required argument --type" \
    "$TOOL/assemble.sh" --type "$bad" --global-project-id "$SYN_PROJECT" --output-dir "$T/rej" --shared-crds-output "$T/rej-shared.yaml"
  check "no output created for rejected type '${bad:-<empty>}'" bash -c "[[ ! -e '$T/rej' && ! -e '$T/rej-shared.yaml' ]]"
done
expect_fail "global is explicitly out of scope" "global bundle is out of scope" \
  "$TOOL/assemble.sh" --type global --global-project-id "$SYN_PROJECT" --output-dir "$T/rej"
expect_fail "existing output directory is never overwritten" "already exists" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --output-dir "$T/mc"
expect_fail "instance name 'argocd' collides with the ESO argocd-cluster Secret" "collides with a parent/day-two identity" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --instance-name argocd --output-dir "$T/rej2"
expect_fail "instance name colliding with a parent identity is rejected" "collides with a parent/day-two identity" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --instance-name argocd-secrets --output-dir "$T/rej3"
expect_fail "over-long instance name is rejected" "at most 29" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --instance-name "$(printf 'a%.0s' {1..30})" --output-dir "$T/rej4"
expect_fail "operator namespace equal to the instance namespace is rejected" "must differ" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --namespace argocd --output-dir "$T/rej5"
expect_fail "operator namespace argocd is rejected (parent-owned)" "must differ|must not be argocd" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --namespace argocd --instance-namespace other --output-dir "$T/rej6"
for badp in Synthetic-Project short 1synthetic-project synthetic-project- "synthetic_project"; do
  expect_fail "global project id '$badp' is rejected" "not a valid GCP project ID" \
    "$TOOL/assemble.sh" --type mc --global-project-id "$badp" --output-dir "$T/rej7"
done
check "no output directory was created by any rejected assembly" bash -c "! ls '$T' | grep -q '^rej[0-9]*\$'"

# -------------------------------------------------- bundle semantics
kubectl kustomize "$T/mc" > "$T/mc-built.yaml" 2>/dev/null
j "$T/mc-built.yaml" > "$T/mc-built.json"
ids() { jq -r '.[] | [.kind, (.metadata.namespace // "-"), .metadata.name] | join("/")' "$1" | LC_ALL=C sort; }
cat > "$T/expected-ids.txt" <<'EOF'
ArgoCD/argocd/gitops
ClusterRole/-/gitops-application-controller-interim-parity
ClusterRole/-/openshift-gitops-operator-controller-manager
ClusterRoleBinding/-/gitops-application-controller-interim-parity
ClusterRoleBinding/-/openshift-gitops-operator-controller-manager
CustomResourceDefinition/-/argocds.argoproj.io
CustomResourceDefinition/-/controllerconfigurations.promoter.argoproj.io
CustomResourceDefinition/-/namespacemanagements.argoproj.io
CustomResourceDefinition/-/notificationsconfigurations.argoproj.io
CustomResourceDefinition/-/rolloutmanagers.argoproj.io
Deployment/gitops-operator/openshift-gitops-operator-controller-manager
ExternalSecret/gitops-operator/redhat-registry-pull-secret
Namespace/-/gitops-operator
NetworkPolicy/gitops-operator/operator-default-deny-ingress
Role/gitops-operator/openshift-gitops-operator-leader-election
RoleBinding/gitops-operator/openshift-gitops-operator-leader-election
SecretStore/gitops-operator/gitops-registry-bootstrap-gcpsm
ServiceAccount/gitops-operator/openshift-gitops-operator-controller-manager
EOF
check "child inventory is exactly the 18 reviewed identities" bash -c "diff <(sed -n '1,\$p' '$T/expected-ids.txt' | LC_ALL=C sort) <(jq -r '.[] | [.kind, (.metadata.namespace // \"-\"), .metadata.name] | join(\"/\")' '$T/mc-built.json' | LC_ALL=C sort)"
check "MC and region both carry exactly one ingress-only default-deny NetworkPolicy (no Egress isolation, no allow rules)" \
  bash -c "for t in mc region; do kubectl kustomize '$T/'\$t | yq eval -o=json -I=0 '.' - | jq -se '[.[] | select(.kind==\"NetworkPolicy\")] | length == 1 and .[0].metadata.name == \"operator-default-deny-ingress\" and .[0].metadata.namespace == \"gitops-operator\" and .[0].spec == {podSelector:{}, policyTypes:[\"Ingress\"], ingress:[]}' >/dev/null || exit 1; done"
check "the empty podSelector selects the manager: same namespace, not hostNetwork, no other NetworkPolicy" \
  bash -c "jq -e '([.[] | select(.kind==\"NetworkPolicy\")][0].metadata.namespace) as \$ns | [.[] | select(.kind==\"Deployment\")] | length == 1 and .[0].metadata.namespace == \$ns and (.[0].spec.template.spec.hostNetwork // false) == false' '$T/mc-built.json'"
check "the NetworkPolicy adds no depends-on edge and carries no lifecycle annotation" \
  bash -c "jq -e '.[] | select(.kind==\"NetworkPolicy\") | (.metadata.annotations // {}) | length == 0' '$T/mc-built.json'"
check "child declares none of the parent-owned identities (argocd Namespace, ESO, stores, metadata template, root-app, shared CRDs)" \
  bash -c "jq -e 'all(.[];
      (.kind == \"Namespace\" and .metadata.name == \"argocd\" | not)
      and ((.kind | IN(\"ApplicationSet\",\"Application\",\"AppProject\",\"ClusterSecretStore\",\"ConfigMap\",\"Secret\")) | not)
      and (.metadata.name | IN(\"argocd-local-gcpsm\",\"argocd-secrets\",\"argocd-cluster\",\"argocd-git-credentials\",\"cluster-metadata-annotations-template\",\"root-app\",\"argocd-global-secret-store\",\"argocd-oauth-secret-accessor\") | not)
      and ((.metadata.namespace != \"argocd\") or .kind == \"ArgoCD\")
      and ((.metadata.name | IN(\"applications.argoproj.io\",\"applicationsets.argoproj.io\",\"appprojects.argoproj.io\")) | not)
      and ((.metadata.name | test(\"external-secrets.io\$\")) | not))' '$T/mc-built.json'"
check "child declares no argocd-cm seed and no ConfigMap at all" bash -c "jq -e 'all(.[]; .kind != \"ConfigMap\")' '$T/mc-built.json'"
check "child declares none of the excluded independent/optional CRDs" \
  bash -c "jq -e 'all(.[] | select(.kind==\"CustomResourceDefinition\"); .metadata.name | IN(\"rollouts.argoproj.io\",\"analysisruns.argoproj.io\",\"analysistemplates.argoproj.io\",\"clusteranalysistemplates.argoproj.io\",\"experiments.argoproj.io\",\"imageupdaters.argocd-image-updater.argoproj.io\",\"gitopsservices.pipelines.openshift.io\",\"promotionstrategies.promoter.argoproj.io\",\"changetransferpolicies.promoter.argoproj.io\",\"scmproviders.promoter.argoproj.io\",\"gitrepositories.promoter.argoproj.io\",\"pullrequests.promoter.argoproj.io\",\"clusterscmproviders.promoter.argoproj.io\",\"commitstatuses.promoter.argoproj.io\",\"argocdcommitstatuses.promoter.argoproj.io\",\"gitcommitstatuses.promoter.argoproj.io\",\"revertcommits.promoter.argoproj.io\",\"scheduledcommitstatuses.promoter.argoproj.io\",\"timedcommitstatuses.promoter.argoproj.io\",\"webrequestcommitstatuses.promoter.argoproj.io\") | not)' '$T/mc-built.json'"
check "child CRDs (5) and shared CRDs (3) are disjoint and the parent artifact has no ControllerConfiguration" \
  bash -c "c=\$(yq eval '.metadata.name' '$T/mc/crds.yaml' | grep -v '^---\$' | sort); s=\$(yq eval '.metadata.name' '$T/mc-shared.yaml' | grep -v '^---\$' | sort); (( \$(wc -l <<<\"\$c\") == 5 && \$(wc -l <<<\"\$s\") == 3 )) && [[ -z \$(comm -12 <(echo \"\$c\") <(echo \"\$s\")) ]] && ! grep -q controllerconfigurations '$T/mc-shared.yaml' && ! grep -qE 'name: (applications|applicationsets|appprojects)\\.argoproj\\.io\$' '$T/mc/crds.yaml'"
check "no child CRD declares a conversion webhook; ArgoCD is v1beta1-only" \
  bash -c "jq -e 'all(.[] | select(.kind==\"CustomResourceDefinition\"); .spec.conversion == null or .spec.conversion.strategy == \"None\") and (.[] | select(.metadata.name==\"argocds.argoproj.io\") | (.spec.versions | map(.name)) == [\"v1beta1\"])' '$T/mc-built.json'"
check "ControllerConfiguration CRD spec equals the official bundle schema (only annotations added)" \
  bash -c "diff <(yq eval -o=json '.spec' '$T/x/gitops-operator-crds/crds/controllerconfigurations.promoter.argoproj.io.yaml' | jq -S .) <(jq -S '.[] | select(.metadata.name==\"controllerconfigurations.promoter.argoproj.io\") | .spec' '$T/mc-built.json')"
# Exact lifecycle annotation sets: ArgoCD seed = {mutation:ignore, deletion:detach};
# ControllerConfiguration CRD = {deletion:detach} only (mutation-managed); nothing else.
check "lifecycle annotation sets are exact: seed ignore+detach, ControllerConfiguration CRD detach only, all else none" \
  bash -c "diff <(jq -r '.[] | select(.metadata.annotations[\"client.lifecycle.config.k8s.io/mutation\"] != null or .metadata.annotations[\"client.lifecycle.config.k8s.io/deletion\"] != null) | .kind+\"/\"+.metadata.name+\"=\"+(.metadata.annotations[\"client.lifecycle.config.k8s.io/mutation\"] // \"-\")+\",\"+(.metadata.annotations[\"client.lifecycle.config.k8s.io/deletion\"] // \"-\")' '$T/mc-built.json' | sort) <(printf 'ArgoCD/gitops=ignore,detach\nCustomResourceDefinition/controllerconfigurations.promoter.argoproj.io=-,detach\n')"
check "no other child CRD, the manager or the operator namespace is detach-protected (retirement stays destructive)" \
  bash -c "jq -e 'all(.[] | select(.kind==\"Deployment\" or .kind==\"Namespace\" or (.kind==\"CustomResourceDefinition\" and .metadata.name != \"controllerconfigurations.promoter.argoproj.io\")); .metadata.annotations[\"client.lifecycle.config.k8s.io/deletion\"] == null and .metadata.annotations[\"client.lifecycle.config.k8s.io/mutation\"] == null)' '$T/mc-built.json'"
check "the actual final depends-on graph is valid (edges present, resolvable, no mutation:ignore target, acyclic)" \
  python3 -I "$TOOL/check-depends-on.py" "$T/mc-built.json"
check "ArgoCD seed depends on the argocds and ControllerConfiguration CRDs and the manager (API before CR)" \
  bash -c "jq -e '.[] | select(.kind==\"ArgoCD\") | .metadata.annotations[\"config.kubernetes.io/depends-on\"] | split(\",\") | (index(\"apiextensions.k8s.io/CustomResourceDefinition/controllerconfigurations.promoter.argoproj.io\") != null) and (index(\"apiextensions.k8s.io/CustomResourceDefinition/argocds.argoproj.io\") != null) and (index(\"apps/namespaces/gitops-operator/Deployment/openshift-gitops-operator-controller-manager\") != null)' '$T/mc-built.json'"
check "manager depends on all five child CRDs (API before watcher) and the registry ExternalSecret" \
  bash -c "jq -e '.[] | select(.kind==\"Deployment\") | .metadata.annotations[\"config.kubernetes.io/depends-on\"] | split(\",\") | length == 6 and (map(select(startswith(\"apiextensions.k8s.io/CustomResourceDefinition/\"))) | length == 5) and (index(\"external-secrets.io/namespaces/gitops-operator/ExternalSecret/redhat-registry-pull-secret\") != null)' '$T/mc-built.json'"

# Negative regressions for the dependency-graph checker, on copies of the real built graph.
CC_REF='apiextensions.k8s.io/CustomResourceDefinition/controllerconfigurations.promoter.argoproj.io'
DEP_REF='apps/namespaces/gitops-operator/Deployment/openshift-gitops-operator-controller-manager'
ARGO_REF='argoproj.io/namespaces/argocd/ArgoCD/gitops'
ES_REF2='external-secrets.io/namespaces/gitops-operator/ExternalSecret/redhat-registry-pull-secret'
SS_REF='external-secrets.io/namespaces/gitops-operator/SecretStore/gitops-registry-bootstrap-gcpsm'
mutate() { # mutate <out> <jq program>
  jq "$2" "$T/mc-built.json" > "$1"
}
add_dep() { # add_dep <kind> <ref>  -> jq program appending a depends-on token to every object of that kind
  echo "map(if .kind == \"$1\" then .metadata.annotations[\"config.kubernetes.io/depends-on\"] = ((.metadata.annotations[\"config.kubernetes.io/depends-on\"] // \"\") + (if (.metadata.annotations[\"config.kubernetes.io/depends-on\"] // \"\") == \"\" then \"\" else \",\" end) + \"$2\") else . end)"
}
mutate "$T/g-zero.json" 'map(del(.metadata.annotations["config.kubernetes.io/depends-on"]))'
expect_fail "graph checker fails on zero depends-on edges" "no depends-on edge" python3 -I "$TOOL/check-depends-on.py" "$T/g-zero.json"
mutate "$T/g-cycle2.json" "$(add_dep Deployment "$ARGO_REF")"
expect_fail "graph checker catches the Deployment<->ArgoCD cycle" "dependency cycle" python3 -I "$TOOL/check-depends-on.py" "$T/g-cycle2.json"
mutate "$T/g-cycle2b.json" "$(add_dep ExternalSecret "$DEP_REF")"
expect_fail "graph checker catches the Deployment<->ExternalSecret cycle" "dependency cycle" python3 -I "$TOOL/check-depends-on.py" "$T/g-cycle2b.json"
mutate "$T/g-cycle4.json" "$(add_dep SecretStore "$ARGO_REF")"
expect_fail "graph checker catches a longer cycle (ArgoCD->manager->ExternalSecret->SecretStore->ArgoCD)" "dependency cycle" python3 -I "$TOOL/check-depends-on.py" "$T/g-cycle4.json"
mutate "$T/g-selfcycle.json" "$(add_dep SecretStore "$SS_REF")"
expect_fail "graph checker catches a self dependency" "dependency cycle" python3 -I "$TOOL/check-depends-on.py" "$T/g-selfcycle.json"
mutate "$T/g-ignored-cc.json" 'map(if .metadata.name == "controllerconfigurations.promoter.argoproj.io" then .metadata.annotations["client.lifecycle.config.k8s.io/mutation"] = "ignore" else . end)'
expect_fail "graph checker rejects a mutation:ignore ControllerConfiguration target" "mutation:ignore" python3 -I "$TOOL/check-depends-on.py" "$T/g-ignored-cc.json"
mutate "$T/g-ignored-argo.json" "$(add_dep Role "$ARGO_REF")"
expect_fail "graph checker rejects a dependent of the mutation:ignore ArgoCD seed" "mutation:ignore" python3 -I "$TOOL/check-depends-on.py" "$T/g-ignored-argo.json"
mutate "$T/g-unresolved.json" "$(add_dep Deployment "apps/namespaces/gitops-operator/Deployment/does-not-exist")"
expect_fail "graph checker rejects an unresolved token" "not declared in the bundle" python3 -I "$TOOL/check-depends-on.py" "$T/g-unresolved.json"
mutate "$T/g-malformed.json" "$(add_dep Deployment "not/a/valid/token")"
expect_fail "graph checker rejects a malformed token" "malformed depends-on token" python3 -I "$TOOL/check-depends-on.py" "$T/g-malformed.json"

# The assembler itself must reject these graphs before publishing any output.
copy_templates() { rm -rf "$1"; cp -R "$TOOL/bundle-templates" "$1"; }
copy_templates "$T/tpl-ignore"
python3 -I - "$T/tpl-ignore/kustomization.yaml.tmpl" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read()
s = s.replace("          client.lifecycle.config.k8s.io/deletion: detach\n",
              "          client.lifecycle.config.k8s.io/mutation: ignore\n          client.lifecycle.config.k8s.io/deletion: detach\n", 1)
open(p, "w").write(s)
PY
expect_fail "assembler rejects a template that marks the ControllerConfiguration CRD mutation:ignore" "lifecycle annotations are not exactly|invalid depends-on graph" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --template-dir "$T/tpl-ignore" --output-dir "$T/tpl-ignore-out" --shared-crds-output "$T/tpl-ignore-shared.yaml"
check "no output or shared artifact was published for the rejected ignore template" bash -c "[[ ! -e '$T/tpl-ignore-out' && ! -e '$T/tpl-ignore-shared.yaml' ]]"
copy_templates "$T/tpl-cycle"
sed -i.bak 's|__MANAGER_DEPENDS_ON__|__MANAGER_DEPENDS_ON__,argoproj.io/namespaces/__INSTANCE_NAMESPACE__/ArgoCD/__INSTANCE_NAME__|' "$T/tpl-cycle/kustomization.yaml.tmpl" && rm -f "$T/tpl-cycle/kustomization.yaml.tmpl.bak"
expect_fail "assembler rejects a template creating a Deployment<->ArgoCD cycle" "invalid depends-on graph" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --template-dir "$T/tpl-cycle" --output-dir "$T/tpl-cycle-out"
check "no output was published for the rejected cycle template" test ! -e "$T/tpl-cycle-out"
copy_templates "$T/tpl-nodeps"
sed -i.bak '/depends-on/d' "$T/tpl-nodeps/registry.yaml.tmpl" "$T/tpl-nodeps/gitops-instance.yaml.tmpl" "$T/tpl-nodeps/kustomization.yaml.tmpl" && rm -f "$T"/tpl-nodeps/*.bak
expect_fail "assembler rejects a bundle with zero depends-on edges" "invalid depends-on graph" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --template-dir "$T/tpl-nodeps" --output-dir "$T/tpl-nodeps-out"
check "no output was published for the zero-edge template" test ! -e "$T/tpl-nodeps-out"
copy_templates "$T/tpl-egress"
python3 -I - "$T/tpl-egress/networkpolicy.yaml.tmpl" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read()
s = s.replace("    - Ingress\n", "    - Ingress\n    - Egress\n")
open(p, "w").write(s)
PY
expect_fail "assembler rejects a NetworkPolicy template that adds Egress isolation" "not exactly the ingress-only default deny" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --template-dir "$T/tpl-egress" --output-dir "$T/tpl-egress-out" --shared-crds-output "$T/tpl-egress-shared.yaml"
check "nothing was published for the Egress template" bash -c "[[ ! -e '$T/tpl-egress-out' && ! -e '$T/tpl-egress-shared.yaml' ]]"
copy_templates "$T/tpl-allow"
python3 -I - "$T/tpl-allow/networkpolicy.yaml.tmpl" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read()
s = s.replace("  ingress: []\n", "  ingress:\n    - from:\n        - podSelector: {}\n")
open(p, "w").write(s)
PY
expect_fail "assembler rejects a NetworkPolicy template with an ingress allow rule" "not exactly the ingress-only default deny" \
  "$TOOL/assemble.sh" --type mc --global-project-id "$SYN_PROJECT" --template-dir "$T/tpl-allow" --output-dir "$T/tpl-allow-out" --shared-crds-output "$T/tpl-allow-shared.yaml"
check "nothing was published for the allow-rule template" bash -c "[[ ! -e '$T/tpl-allow-out' && ! -e '$T/tpl-allow-shared.yaml' ]]"

# seed settings, validated against the actual CRD schema
yq eval -o=json -I=0 'select(.kind=="ArgoCD")' "$T/mc/gitops-instance.yaml" > "$T/seed.json"
yq eval -o=json -I=0 '.' "$T/mc/crds.yaml" > "$T/child-crds.json"
check "ArgoCD seed validates against the v1beta1 CRD schema (no unknown/pruned fields)" python3 -I "$HERE/validate_cr.py" "$T/child-crds.json" argocds.argoproj.io v1beta1 "$T/seed.json"
sed 's/"disableAdmin"/"disableAdminn"/' "$T/seed.json" > "$T/seed-bad.json"
expect_fail "schema validator rejects a mistyped seed field" "unknown field disableAdminn" python3 -I "$HERE/validate_cr.py" "$T/child-crds.json" argocds.argoproj.io v1beta1 "$T/seed-bad.json"
check "seed enables ApplicationSet, Notifications and Dex; admin and anonymous disabled; annotation tracking" \
  bash -c "jq -e '.spec.applicationSet.enabled == true and .spec.notifications.enabled == true and .spec.sso.provider == \"dex\" and .spec.disableAdmin == true and .spec.usersAnonymousEnabled == false and .spec.resourceTrackingMethod == \"annotation\" and .metadata.name == \"gitops\" and .metadata.namespace == \"argocd\"' '$T/seed.json'"
check "seed Dex config equals the existing bootstrap argocd-cm authproxy seed" \
  bash -c "diff <(yq eval '.data.\"dex.config\"' '$ROOT/bootstrap/argocd/argocd-cm-patch.yaml') <(yq eval '.spec.sso.dex.config' '$T/mc/gitops-instance.yaml')"
check "operator-created '<instance>-cluster' Secret cannot collide with the ESO argocd-cluster Secret" \
  bash -c "[[ gitops-cluster != argocd-cluster ]] && grep -q 'name: argocd-cluster' '$ROOT/bootstrap/argocd/external-secret-argocd-cluster.yaml'"

# registry store/secret
ESO="$ROOT/bootstrap/external-secrets/controller.yaml"
yq eval -o=json -I=0 'select(.kind=="CustomResourceDefinition" and (.metadata.name=="secretstores.external-secrets.io" or .metadata.name=="externalsecrets.external-secrets.io"))' "$ESO" > "$T/eso-crds.json"
yq eval -o=json -I=0 'select(.kind=="SecretStore")' "$T/mc/registry.yaml" > "$T/ss.json"
yq eval -o=json -I=0 'select(.kind=="ExternalSecret")' "$T/mc/registry.yaml" > "$T/es.json"
check "SecretStore validates against the bootstrap ESO CRD schema" python3 -I "$HERE/validate_cr.py" "$T/eso-crds.json" secretstores.external-secrets.io v1 "$T/ss.json"
check "ExternalSecret validates against the bootstrap ESO CRD schema" python3 -I "$HERE/validate_cr.py" "$T/eso-crds.json" externalsecrets.external-secrets.io v1 "$T/es.json"
check "registry store uses the explicit synthetic global project and the ESO controller identity (no auth stanza)" \
  bash -c "jq -e '.spec.provider.gcpsm.projectID == \"$SYN_PROJECT\" and (.spec.provider.gcpsm | has(\"auth\") | not) and .metadata.namespace == \"gitops-operator\" and .metadata.name == \"gitops-registry-bootstrap-gcpsm\"' '$T/ss.json'"
check "registry ExternalSecret: dockerconfigjson type, propagate label, no decoding, operator namespace, bootstrap store" \
  bash -c "jq -e '.spec.target.template.type == \"kubernetes.io/dockerconfigjson\" and .spec.target.template.metadata.labels[\"operator.argoproj.io/propagate-image-pull-secret\"] == \"true\" and .spec.data[0].remoteRef.decodingStrategy == \"None\" and .spec.data[0].remoteRef.key == \"default-openshift-pull-secret\" and .spec.secretStoreRef.name == \"gitops-registry-bootstrap-gcpsm\" and .spec.secretStoreRef.kind == \"SecretStore\" and .metadata.namespace == \"gitops-operator\" and .spec.target.name == \"redhat-registry-pull-secret\"' '$T/es.json'"
check "registry store name differs from the bootstrap and day-two stores and KSAs" \
  bash -c "! grep -qE 'argocd-local-gcpsm|argocd-global-secret-store|argocd-oauth-secret-accessor|argocd-secrets' '$T/mc/registry.yaml'"
check "no cluster project/name/location or credential key material anywhere in the bundle" \
  bash -c "! grep -rEi 'clusterProjectID|clusterName|clusterLocation|serviceAccountKey|secretRef|\"auths\"|PRIVATE KEY|password:' '$T/mc' --include='*.yaml' --exclude=crds.yaml"
check "no data or stringData and no Secret object in any bundle file" \
  bash -c "! grep -rEq '^(data|stringData):' '$T/mc/registry.yaml' '$T/mc/gitops-instance.yaml' '$T/mc/controller-rbac.yaml' '$T/mc/operator.yaml' && jq -e 'all(.[]; .kind != \"Secret\")' '$T/mc-built.json'"
check "manager Deployment in the built bundle references the pull secret and its ESO-created Secret name" \
  bash -c "jq -e '.[] | select(.kind==\"Deployment\") | (.spec.template.spec.imagePullSecrets | map(.name)) == [\"redhat-registry-pull-secret\"]' '$T/mc-built.json'"

# interim controller RBAC
check "interim controller RBAC mirrors the legacy application-controller wildcard rules under a distinct name and the operator SA" \
  bash -c "diff <(jq -S '.[] | select(.kind==\"ClusterRole\" and .metadata.name==\"gitops-application-controller-interim-parity\") | .rules' '$T/mc-built.json') <(yq eval -o=json 'select(.kind==\"ClusterRole\" and .metadata.name==\"argocd-application-controller\") | .rules' '$ROOT/bootstrap/argocd/install.yaml' | jq -S .) && jq -e '.[] | select(.kind==\"ClusterRoleBinding\" and .metadata.name==\"gitops-application-controller-interim-parity\") | .subjects == [{kind:\"ServiceAccount\", name:\"gitops-argocd-application-controller\", namespace:\"argocd\"}]' '$T/mc-built.json'"

check "both final bundles carry the exact manager resource block" \
  bash -c "for t in mc region; do kubectl kustomize '$T/'\$t | yq eval -o=json -I=0 '.' - | jq -se '.[] | select(.kind==\"Deployment\") | .spec.template.spec.containers[0].resources == {requests:{cpu:\"100m\",memory:\"256Mi\"}, limits:{cpu:\"1\",memory:\"1Gi\"}}' >/dev/null || exit 1; done"
check "interim RBAC is documented as a temporary additive exception with the GCP-1285 exit criterion (template, README, PROVENANCE)" \
  bash -c "for f in '$TOOL/bundle-templates/controller-rbac.yaml.tmpl' '$ROOT/bootstrap-gitops/README.md' '$TOOL/chart-src/PROVENANCE.md'; do grep -q 'GCP-1285' \"\$f\" && grep -qi 'additive' \"\$f\" || exit 1; done; grep -q 'ReconcilerHook' '$TOOL/bundle-templates/controller-rbac.yaml.tmpl'; ! grep -q 'mirrors the wildcard permissions of the existing application controller until' '$TOOL/bundle-templates/controller-rbac.yaml.tmpl'"
check "resources budget is documented as untuned in README, PROVENANCE and the packaged NOTICE, with the Dev AIO observation gate" \
  bash -c "grep -qi 'untuned' '$ROOT/bootstrap-gitops/README.md' && grep -q 'OOM' '$ROOT/bootstrap-gitops/README.md' && grep -qi 'untuned' '$TOOL/chart-src/PROVENANCE.md' && grep -qi 'UNTUNED' '$T/x/gitops-operator/NOTICE' && grep -qi 'UNTUNED' '$T/x/gitops-operator-crds/NOTICE'"

# ---- synthetic regressions for the bootstrap/ guard, in disposable repos under $T
fx() { # fx <repo> <git args...>: hermetic local identity, no signing, no hooks
  local d="$1"; shift
  env GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null GIT_AUTHOR_NAME=fixture GIT_AUTHOR_EMAIL=fixture@example.invalid \
    GIT_COMMITTER_NAME=fixture GIT_COMMITTER_EMAIL=fixture@example.invalid \
    git -C "$d" -c commit.gpgsign=false -c core.hooksPath=/dev/null "$@"
}
new_fixture() { # new_fixture <dir>: main with bootstrap/a.txt and other/x.txt, then branch feature
  rm -rf "$1"; mkdir -p "$1"
  fx "$1" init -q -b main
  mkdir "$1/bootstrap" "$1/other"; echo base > "$1/bootstrap/a.txt"; echo base > "$1/other/x.txt"
  fx "$1" add -A; fx "$1" commit -q -m base
  fx "$1" checkout -q -b feature
}
fixture_guard() { # fixture_guard <dir> <ref>: the real helpers, exactly as the suite uses them
  local b
  b="$(resolve_bootstrap_base "$1" "$2")" || return 1
  bootstrap_unchanged "$1" "$b"
}
FXD="$T/fx"
new_fixture "$FXD"
check "guard passes on a clean synthetic bootstrap" fixture_guard "$FXD" main
echo more > "$FXD/other/y.txt"; fx "$FXD" add -A; fx "$FXD" commit -q -m unrelated
check "guard passes with only an unrelated committed change" fixture_guard "$FXD" main
check "guard also accepts the base given as a commit SHA" fixture_guard "$FXD" "$(fx "$FXD" rev-parse main)"
echo changed > "$FXD/bootstrap/a.txt"; fx "$FXD" add -A; fx "$FXD" commit -q -m "edit bootstrap"
check "fixture: the committed bootstrap edit leaves the working tree clean (the old HEAD-only guard would pass)" \
  bash -c "[[ -z \$(git -C '$FXD' status --porcelain) ]] && git -C '$FXD' diff --quiet HEAD -- bootstrap"
expect_fail "guard fails on a COMMITTED bootstrap edit even with a clean working tree" "differs from merge base" fixture_guard "$FXD" main
new_fixture "$FXD"
echo wip >> "$FXD/bootstrap/a.txt"
expect_fail "guard fails on an unstaged bootstrap edit" "differs from merge base" fixture_guard "$FXD" main
fx "$FXD" add -A
expect_fail "guard fails on a staged bootstrap edit" "differs from merge base" fixture_guard "$FXD" main
new_fixture "$FXD"
echo new > "$FXD/bootstrap/untracked.txt"
expect_fail "guard fails on an untracked bootstrap file" "untracked changes" fixture_guard "$FXD" main
new_fixture "$FXD"
expect_fail "guard fails clearly on an unresolvable base ref" "does not resolve to a commit" fixture_guard "$FXD" no-such/main
expect_fail "guard treats an option-looking base ref as unresolvable, not as a flag" "does not resolve to a commit" fixture_guard "$FXD" --all
fx "$FXD" branch unrelated "$(fx "$FXD" commit-tree "$(fx "$FXD" hash-object -t tree -w /dev/null)" -m orphan)"
expect_fail "guard fails clearly when the base shares no history with HEAD" "shares no history" fixture_guard "$FXD" unrelated
# The runner itself aborts before any test, quickly and without network, for a bad base.
BOOTSTRAP_BASE_REF=no-such/base "$HERE/run-step2-tests.sh" > "$T/bad-base.out" 2>&1; bad_rc=$?
check "the runner aborts early (nonzero) for a nonexistent BOOTSTRAP_BASE_REF" test "$bad_rc" -ne 0
check "the early abort names the base problem, says it aborted before any test, and ran no test" \
  bash -c "grep -q 'does not resolve to a commit' '$T/bad-base.out' && grep -q 'aborting before any test' '$T/bad-base.out' && ! grep -q '^ok ' '$T/bad-base.out'"

# --------------------------------------------------------- repo-level
check "committed MC bundle equals a fresh deterministic assembly" \
  bash -c "diff -r '$ROOT/bootstrap-gitops/workloads/synthetic/mc' '$T/mc'"
check "committed region bundle equals a fresh deterministic assembly" \
  bash -c "diff -r '$ROOT/bootstrap-gitops/workloads/synthetic/region' '$T/region'"
check "committed shared-CRD parent artifact equals a fresh extraction" \
  cmp "$ROOT/bootstrap-gitops/parent-artifacts/shared-crds.yaml" "$T/mc-shared.yaml"
check "committed bundles use only the synthetic project and no real identifiers" \
  bash -c "! grep -rEi 'gcp-hcp-commons|rflores|-gke|dev-mgt|dev-reg' '$ROOT/bootstrap-gitops' '$TOOL/pins.yaml' '$TOOL/bundle-templates' --include='*.yaml' --include='*.tmpl'"
check "active bootstrap/ is unchanged since the merge base with $BOOTSTRAP_BASE_REF (committed, staged, unstaged, untracked)" \
  bootstrap_unchanged "$ROOT" "$BOOTSTRAP_BASE_COMMIT"
check "nothing outside the new subtrees references bootstrap-gitops (inactive)" \
  bash -c "cd '$ROOT' && ! grep -rIl 'bootstrap-gitops/' --exclude-dir=.git --exclude-dir=bootstrap-gitops --exclude-dir=gitops-bootstrap . | grep -q ."
check "shell scripts pass bash -n" \
  bash -c "for f in '$TOOL/assemble.sh' '$TOOL/render.sh' '$TOOL/chart-src/build-charts.sh' '$HERE/run-step2-tests.sh'; do bash -n \"\$f\" || exit 1; done"
check "python helpers parse" \
  bash -c "for f in '$TOOL/chart-src/package-chart.py' '$HERE/validate_cr.py' '$TOOL/check-depends-on.py'; do python3 -I -c 'import ast,sys; ast.parse(open(sys.argv[1]).read())' \"\$f\" || exit 1; done"

echo
echo "step2 results: $PASS passed, $FAIL failed"
[[ "$FAIL" -eq 0 ]]
