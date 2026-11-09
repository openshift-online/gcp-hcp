#!/usr/bin/env bash
# Assemble an INACTIVE, offline MC or region workload bundle (GCP-1280 Step 2)
# from render.sh output plus the templates in bundle-templates/.
#
# Never contacts a network or a cluster. Takes synthetic or real GCP global
# project IDs as plain input and writes nothing outside the new output
# directory (and the optional shared-CRD artifact file). Only the types
# `mc` and `region` exist: any other value (including `global`) is rejected
# before any work happens.
#
# See ../../bootstrap-gitops/README.md for the ownership model and
# README.md in this directory for the renderer contract it builds on.
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: assemble.sh --type mc|region --global-project-id <project-id> \
         --output-dir <new-dir> [--shared-crds-output <new-file>] \
         [--pins FILE] [--image-allowlist FILE] [--template-dir DIR] \
         [--namespace NAME] [--instance-name NAME] [--instance-namespace NAME]

  --type                 mc or region. Anything else (global, unknown) is rejected.
  --global-project-id    Global GCP project ID holding the registry pull secret
                          (ESO controller identity reads it; no cluster IDs/keys).
  --output-dir           New directory for the self-contained bundle (must not exist).
  --shared-crds-output   Optional new file: the three shared Application/
                          ApplicationSet/AppProject CRDs for the later parent.
  --pins                 Default: pins.yaml next to this script.
  --image-allowlist      Default: allowed-images.txt next to this script.
  --template-dir         Default: bundle-templates next to this script (used by
                          the tests to prove the fail-closed checks).
  --namespace            Operator namespace. Default: gitops-operator.
  --instance-name        ArgoCD instance name. Default: gitops. Must not be
                          `argocd` (the operator would create an `argocd-cluster`
                          admin Secret colliding with the ESO metadata Secret).
  --instance-namespace   Argo CD instance namespace. Default: argocd.
  --controller-cluster-role  Optional reviewed controller namespace role.
EOF
}

die() { echo "assemble.sh: $*" >&2; exit 1; }
require_tool() { command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"; }

valid_dns_label() { [[ "$1" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ && ${#1} -le 63 ]]; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TYPE=""
GLOBAL_PROJECT_ID=""
OUTPUT_DIR=""
SHARED_CRDS_OUTPUT=""
PINS_FILE="$SCRIPT_DIR/pins.yaml"
ALLOWLIST_FILE="$SCRIPT_DIR/allowed-images.txt"
TEMPLATE_DIR="$SCRIPT_DIR/bundle-templates"
NAMESPACE="gitops-operator"
INSTANCE_NAME="gitops"
INSTANCE_NAMESPACE="argocd"
CONTROLLER_CLUSTER_ROLE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --type) TYPE="$2"; shift 2 ;;
    --global-project-id) GLOBAL_PROJECT_ID="$2"; shift 2 ;;
    --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
    --shared-crds-output) SHARED_CRDS_OUTPUT="$2"; shift 2 ;;
    --pins) PINS_FILE="$2"; shift 2 ;;
    --image-allowlist) ALLOWLIST_FILE="$2"; shift 2 ;;
    --template-dir) TEMPLATE_DIR="$2"; shift 2 ;;
    --namespace) NAMESPACE="$2"; shift 2 ;;
    --instance-name) INSTANCE_NAME="$2"; shift 2 ;;
    --instance-namespace) INSTANCE_NAMESPACE="$2"; shift 2 ;;
    --controller-cluster-role) CONTROLLER_CLUSTER_ROLE="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1 (see --help)" ;;
  esac
done

# Target type is the very first decision: only MC and region bundles exist.
case "$TYPE" in
  mc|region) ;;
  global) die "unsupported --type global: a global bundle is out of scope (only mc and region)" ;;
  "") usage; die "missing required argument --type (mc or region)" ;;
  *) die "unsupported --type '$TYPE' (only mc or region)" ;;
esac
[[ -n "$GLOBAL_PROJECT_ID" ]] || { usage; die "missing required argument --global-project-id"; }
[[ -n "$OUTPUT_DIR" ]] || { usage; die "missing required argument --output-dir"; }

require_tool yq
require_tool jq
require_tool kubectl
require_tool python3

# GCP project ID grammar: 6-30 chars, lowercase letters/digits/hyphens,
# starting with a letter and not ending with a hyphen.
[[ "$GLOBAL_PROJECT_ID" =~ ^[a-z][a-z0-9-]{4,28}[a-z0-9]$ ]] \
  || die "--global-project-id is not a valid GCP project ID: $GLOBAL_PROJECT_ID"
for n in "$NAMESPACE" "$INSTANCE_NAMESPACE" "$INSTANCE_NAME"; do
  valid_dns_label "$n" || die "not a valid DNS-1123 label: $n"
done
# The operator abbreviates/hashes derived names above 29 characters.
[[ ${#INSTANCE_NAME} -le 29 ]] || die "--instance-name must be at most 29 characters"
[[ "$NAMESPACE" != "$INSTANCE_NAMESPACE" ]] || die "operator namespace and instance namespace must differ"

readonly SERVICE_ACCOUNT="openshift-gitops-operator-controller-manager"
readonly PULL_SECRET_NAME="redhat-registry-pull-secret"
readonly REGISTRY_STORE_NAME="gitops-registry-bootstrap-gcpsm"
readonly MANAGER_DEPLOYMENT="openshift-gitops-operator-controller-manager"

# Identities owned by the parent or by the day-two configuration; the child
# must never produce or shadow them. The operator creates "<instance>-cluster"
# (admin Secret), which would collide with the ESO-owned argocd-cluster.
RESERVED_NAMES=(argocd argocd-cluster argocd-local-gcpsm argocd-secrets argocd-global-secret-store
  argocd-oauth-secret-accessor argocd-git-credentials root-app)
for r in "${RESERVED_NAMES[@]}"; do
  [[ "$INSTANCE_NAME" != "$r" ]] || die "--instance-name '$INSTANCE_NAME' collides with a parent/day-two identity (the operator's ${INSTANCE_NAME}-cluster Secret would collide with the ESO argocd-cluster Secret)"
  [[ "$REGISTRY_STORE_NAME" != "$r" && "$PULL_SECRET_NAME" != "$r" ]] || die "child registry identity collides with a parent/day-two identity: $r"
done
[[ "$INSTANCE_NAMESPACE" != "kube-system" && "$NAMESPACE" != "kube-system" && "$NAMESPACE" != "argocd" ]] \
  || die "operator namespace must not be argocd or kube-system"

[[ "$OUTPUT_DIR" != "/" ]] || die "refusing to use / as --output-dir"
[[ ! -e "$OUTPUT_DIR" ]] || die "output directory already exists, refusing to overwrite: $OUTPUT_DIR"
[[ -d "$(dirname -- "$OUTPUT_DIR")" ]] || die "parent of --output-dir does not exist"
if [[ -n "$SHARED_CRDS_OUTPUT" ]]; then
  [[ ! -e "$SHARED_CRDS_OUTPUT" ]] || die "shared CRD output already exists, refusing to overwrite: $SHARED_CRDS_OUTPUT"
  [[ -d "$(dirname -- "$SHARED_CRDS_OUTPUT")" ]] || die "parent of --shared-crds-output does not exist"
fi
[[ -f "$PINS_FILE" ]] || die "pins file not found: $PINS_FILE"
[[ -f "$ALLOWLIST_FILE" ]] || die "image allowlist not found: $ALLOWLIST_FILE"
[[ -d "$TEMPLATE_DIR" ]] || die "template directory not found: $TEMPLATE_DIR"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gcp-1280-assemble.XXXXXX")"
trap 'rm -rf "$WORK_DIR"' EXIT

"$SCRIPT_DIR/render.sh" \
  --pins "$PINS_FILE" \
  --output-dir "$WORK_DIR/rendered" \
  --namespace "$NAMESPACE" \
  --instance-namespace "$INSTANCE_NAMESPACE" \
  --controller-cluster-role "$CONTROLLER_CLUSTER_ROLE" \
  --service-account "$SERVICE_ACCOUNT" \
  --image-pull-secret-name "$PULL_SECRET_NAME" \
  --image-allowlist "$ALLOWLIST_FILE" >/dev/null

# CRD ownership split. Exact sets, both directions: a chart that adds or drops
# a CRD fails here rather than silently changing ownership.
CHILD_CRDS=(argocds.argoproj.io controllerconfigurations.promoter.argoproj.io
  namespacemanagements.argoproj.io notificationsconfigurations.argoproj.io rolloutmanagers.argoproj.io)
SHARED_CRDS=(applications.argoproj.io applicationsets.argoproj.io appprojects.argoproj.io)

rendered_crds="$(yq eval '.metadata.name' "$WORK_DIR/rendered/crds.yaml" | grep -v '^---$' | LC_ALL=C sort | tr '\n' ' ')"
expected_crds="$(printf '%s\n' "${CHILD_CRDS[@]}" "${SHARED_CRDS[@]}" | LC_ALL=C sort | tr '\n' ' ')"
[[ "$rendered_crds" == "$expected_crds" ]] || die "rendered CRD set is not exactly the expected child+shared set: $rendered_crds"

select_crds() {
  local expr="" n
  for n in "$@"; do expr+="${expr:+ or }.metadata.name == \"$n\""; done
  yq eval "select($expr)" "$WORK_DIR/rendered/crds.yaml"
}
select_crds "${CHILD_CRDS[@]}" > "$WORK_DIR/crds.yaml"
select_crds "${SHARED_CRDS[@]}" > "$WORK_DIR/shared-crds.yaml"
cp "$WORK_DIR/rendered/operator.yaml" "$WORK_DIR/operator.yaml"

crd_ref() { printf 'apiextensions.k8s.io/CustomResourceDefinition/%s' "$1"; }
CRD_REFS=""
for n in "${CHILD_CRDS[@]}"; do CRD_REFS+="${CRD_REFS:+,}$(crd_ref "$n")"; done
ES_REF="external-secrets.io/namespaces/$NAMESPACE/ExternalSecret/$PULL_SECRET_NAME"
MANAGER_DEPENDS_ON="$CRD_REFS,$ES_REF"
INSTANCE_DEPENDS_ON="$(crd_ref argocds.argoproj.io),$(crd_ref controllerconfigurations.promoter.argoproj.io),apps/namespaces/$NAMESPACE/Deployment/$MANAGER_DEPLOYMENT"

render_template() {
  sed \
    -e "s|__OPERATOR_NAMESPACE__|$NAMESPACE|g" \
    -e "s|__INSTANCE_NAME__|$INSTANCE_NAME|g" \
    -e "s|__INSTANCE_NAMESPACE__|$INSTANCE_NAMESPACE|g" \
    -e "s|__GLOBAL_PROJECT_ID__|$GLOBAL_PROJECT_ID|g" \
    -e "s|__TARGET_TYPE__|$TYPE|g" \
    -e "s|__REGISTRY_STORE_NAME__|$REGISTRY_STORE_NAME|g" \
    -e "s|__PULL_SECRET_NAME__|$PULL_SECRET_NAME|g" \
    -e "s|__MANAGER_DEPENDS_ON__|$MANAGER_DEPENDS_ON|g" \
    -e "s|__INSTANCE_DEPENDS_ON__|$INSTANCE_DEPENDS_ON|g" \
    "$TEMPLATE_DIR/$1.tmpl"
}
for t in registry networkpolicy gitops-instance controller-rbac kustomization; do
  render_template "$t.yaml" > "$WORK_DIR/$t.yaml"
done
rm -rf "$WORK_DIR/rendered"
! grep -q '__[A-Z_]*__' "$WORK_DIR"/registry.yaml "$WORK_DIR"/networkpolicy.yaml "$WORK_DIR"/gitops-instance.yaml \
  "$WORK_DIR"/controller-rbac.yaml "$WORK_DIR"/kustomization.yaml \
  || die "unreplaced template placeholder remains"

kubectl kustomize "$WORK_DIR" > "$WORK_DIR/.preview.yaml"
PREVIEW_JSON="$(yq eval -o=json -I=0 '.' "$WORK_DIR/.preview.yaml" | jq -s '.')"
rm -f "$WORK_DIR/.preview.yaml"

# ---- Semantic validation of the final, kustomized bundle ----

# Exact identity set (kind/namespace/name).
actual_ids="$(jq -r '.[] | [.kind, (.metadata.namespace // "-"), .metadata.name] | join("/")' <<<"$PREVIEW_JSON" | LC_ALL=C sort)"
expected_ids="$(
  {
    printf 'Namespace/-/%s\n' "$NAMESPACE"
    printf 'ServiceAccount/%s/%s\n' "$NAMESPACE" "$SERVICE_ACCOUNT"
    printf 'Deployment/%s/%s\n' "$NAMESPACE" "$MANAGER_DEPLOYMENT"
    printf 'Role/%s/openshift-gitops-operator-leader-election\n' "$NAMESPACE"
    printf 'RoleBinding/%s/openshift-gitops-operator-leader-election\n' "$NAMESPACE"
    printf 'ClusterRole/-/openshift-gitops-operator-controller-manager\n'
    printf 'ClusterRoleBinding/-/openshift-gitops-operator-controller-manager\n'
    for n in "${CHILD_CRDS[@]}"; do printf 'CustomResourceDefinition/-/%s\n' "$n"; done
    printf 'SecretStore/%s/%s\n' "$NAMESPACE" "$REGISTRY_STORE_NAME"
    printf 'ExternalSecret/%s/%s\n' "$NAMESPACE" "$PULL_SECRET_NAME"
    printf 'NetworkPolicy/%s/operator-default-deny-ingress\n' "$NAMESPACE"
    printf 'ArgoCD/%s/%s\n' "$INSTANCE_NAMESPACE" "$INSTANCE_NAME"
    printf 'ClusterRole/-/%s-application-controller-interim-parity\n' "$INSTANCE_NAME"
    printf 'ClusterRoleBinding/-/%s-application-controller-interim-parity\n' "$INSTANCE_NAME"
  } | LC_ALL=C sort
)"
[[ "$actual_ids" == "$expected_ids" ]] || die "bundle identity set differs from the reviewed child inventory"

# The single NetworkPolicy is exactly the ingress-only default deny: no Egress
# isolation, no allow rules, no extra fields.
jq -e --arg ns "$NAMESPACE" '
  [.[] | select(.kind == "NetworkPolicy")]
  | length == 1 and (.[0].metadata.name == "operator-default-deny-ingress") and (.[0].metadata.namespace == $ns)
    and (.[0].spec == {podSelector: {}, policyTypes: ["Ingress"], ingress: []})' <<<"$PREVIEW_JSON" >/dev/null \
  || die "the NetworkPolicy is not exactly the ingress-only default deny"

# No credential-bearing or parent-owned object. The child declares no Secret
# of any kind (the pull secret is created by ESO at runtime).
jq -e 'all(.[]; .kind != "Secret")' <<<"$PREVIEW_JSON" >/dev/null || die "bundle declares a Secret"
jq -e 'all(.[]; (has("data") or has("stringData")) | not)' <<<"$PREVIEW_JSON" >/dev/null || die "bundle declares object data/stringData"

# Recheck on the final composed bundle: every env whose name contains the
# uppercase string IMAGE on every container, and every container image, must be
# an allowlisted reference (render.sh applies the same rule to its own output).
declare -a ALLOWED=()
while IFS= read -r line; do
  [[ -z "$line" || "$line" == \#* ]] && continue
  ALLOWED+=("$line")
done < "$ALLOWLIST_FILE"
allowed_json="$(printf '%s\n' "${ALLOWED[@]}" | jq -R . | jq -s .)"
jq -e --argjson allowed "$allowed_json" '
  [ .[] | select(.kind == "Deployment") | .spec.template.spec
    | ((.containers // []) + (.initContainers // []))[]
    | .image, ((.env // [])[] | select(.name | test("IMAGE")) | .value) ]
  | length > 0 and all(. as $v | $allowed | index($v) != null)' <<<"$PREVIEW_JSON" >/dev/null \
  || die "a container image or *IMAGE* env value is not in the reviewed allowlist"

# Lifecycle annotations: exactly two objects carry any, with exactly these
# sets. The ArgoCD seed is create-if-absent (mutation:ignore + deletion:detach);
# the ControllerConfiguration CRD is mutation-managed and only deletion:detach.
# Nothing else is protected: retiring the child stays destructive and needs a
# separately reviewed sequence.
lifecycle="$(jq -r '.[] | select(.metadata.annotations["client.lifecycle.config.k8s.io/mutation"] != null or .metadata.annotations["client.lifecycle.config.k8s.io/deletion"] != null)
  | .kind + "/" + .metadata.name + "=" + ((.metadata.annotations["client.lifecycle.config.k8s.io/mutation"] // "-") + "," + (.metadata.annotations["client.lifecycle.config.k8s.io/deletion"] // "-"))' <<<"$PREVIEW_JSON" | LC_ALL=C sort | tr '\n' ' ')"
[[ "$lifecycle" == "ArgoCD/$INSTANCE_NAME=ignore,detach CustomResourceDefinition/controllerconfigurations.promoter.argoproj.io=-,detach " ]] \
  || die "lifecycle annotations are not exactly the reviewed sets: $lifecycle"

# The depends-on graph: at least one edge, every token well-formed and
# resolving inside this bundle, no target with mutation:ignore (its dependents
# would be skipped on every sync after the first), and no directed cycle.
printf '%s' "$PREVIEW_JSON" > "$WORK_DIR/.graph.json"
python3 -I "$SCRIPT_DIR/check-depends-on.py" "$WORK_DIR/.graph.json" || die "invalid depends-on graph"
rm -f "$WORK_DIR/.graph.json"

# Every seed setting is exactly the reviewed one (also checked against the
# CRD schema by the test suite).
jq -e --arg ns "$INSTANCE_NAMESPACE" '
  .[] | select(.kind == "ArgoCD")
  | .metadata.namespace == $ns and .spec.disableAdmin == true and .spec.usersAnonymousEnabled == false
    and .spec.resourceTrackingMethod == "annotation" and .spec.applicationSet.enabled == true
    and .spec.notifications.enabled == true and .spec.sso.provider == "dex"' <<<"$PREVIEW_JSON" >/dev/null \
  || die "ArgoCD seed settings differ from the reviewed seed"

[[ ! -e "$OUTPUT_DIR" ]] || die "output directory appeared during assembly: $OUTPUT_DIR"
if [[ -n "$SHARED_CRDS_OUTPUT" ]]; then
  [[ ! -e "$SHARED_CRDS_OUTPUT" ]] || die "shared CRD output appeared during assembly"
  cp "$WORK_DIR/shared-crds.yaml" "$SHARED_CRDS_OUTPUT"
fi
rm -f "$WORK_DIR/shared-crds.yaml"
chmod 755 "$WORK_DIR"
mv "$WORK_DIR" "$OUTPUT_DIR"
trap - EXIT
echo "Assembled inactive $TYPE bundle at $OUTPUT_DIR (not applied, not referenced)"
