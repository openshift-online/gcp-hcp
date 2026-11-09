#!/usr/bin/env bash
# Portable, deterministic, credential-free renderer for the OpenShift GitOps
# operator's CRDs and operator manifests.
#
# Renders from local, checksum-verified Helm chart archives only. Never
# fetches a chart and never requires the GCP-1226 research workspace this
# was adapted from. Derived from the retained research prototype at
# gitops-operator/prototype/update.sh (that script and its sibling YAML are
# read-only research input and are not modified or executed by this tool).
#
# See README.md for the pin/input contract, scope and limitations. The reviewed
# OpenShift GitOps 1.22.2 chart pins are in pins.yaml, with their provenance and
# documented differences in chart-src/PROVENANCE.md; that is engineering pin
# resolution, not certification or runtime proof.
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: render.sh --pins <pins.yaml> --output-dir <dir> \
         --namespace <operator-namespace> \
         --instance-namespace <argocd-instance-namespace> \
         --service-account <name> \
         --image-pull-secret-name <name> \
         (--allowed-image <image-ref> | --image-allowlist <file>)...

Optional:
  --controller-cluster-role NAME  Reviewed controller namespace role; the role
                                  must be supplied separately by its owner.

Required:
  --pins FILE                 Pin/input contract (see pins.example.yaml).
  --output-dir DIR            Destination directory; must not already exist.
  --namespace NAME            Namespace the operator manifests are rendered into
                               (DNS-1123 label, max 63 chars).
  --instance-namespace NAME   Namespace passed as ARGOCD_CLUSTER_CONFIG_NAMESPACES
                               (the supported Argo CD instance namespace; DNS-1123
                               label, max 63 chars).
  --service-account NAME      ServiceAccount name used for the operator Deployment
                               and its token-placeholder Secret (DNS-1123 subdomain,
                               max 253 chars). Must be declared in the rendered
                               output and referenced consistently by the Deployment.
  --image-pull-secret-name NAME
                               Name used for imagePullSecrets[0] (the Secret
                               itself is never rendered; see README.md).
  --allowed-image REF         A digest-pinned image reference approved for use
                               in the rendered output (manager and operand
                               images alike). Repeatable.
  --image-allowlist FILE      File with one allowed image reference per line
                               (blank lines and lines starting with # ignored).

At least one approved image must be supplied via --allowed-image and/or a
non-empty --image-allowlist; there is no default allowlist and no bypass for
omitting it. Every container/initContainer image and every environment
variable on those containers whose name contains the uppercase string IMAGE
(e.g. ARGOCD_IMAGE, RELATED_IMAGE_*, GITOPS_CONSOLE_PLUGIN_IMAGE_PF5) in the
rendered operator manifest must be a literal digest-pinned (...@sha256:<64 hex>)
reference AND present in the supplied allowlist, or the render fails closed.
Such an env var set via valueFrom (not a literal value) also fails closed,
since it cannot be verified statically. Names without the uppercase string
IMAGE (including lowercase variants) are not inspected.
EOF
}

die() {
  echo "render.sh: $*" >&2
  exit 1
}

require_tool() {
  command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

# DNS-1123 label: required for Namespace names (max 63), independent of the
# longer DNS-1123 subdomain rule Kubernetes applies to most other object names.
valid_dns_label() {
  [[ "$1" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ && ${#1} -le 63 ]]
}

# DNS-1123 subdomain (max 253): dot-separated DNS-1123 labels, used for
# ServiceAccount/Secret names and similar (most Kubernetes object names,
# unlike Namespace, allow dots per https://kubernetes.io/docs/concepts/overview/working-with-objects/names/).
valid_dns_subdomain() {
  [[ "$1" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$ && ${#1} -le 253 ]]
}

# Conservative, anchored image-reference grammar for this tool's known chart
# inputs:
#   host[:port] ("/" repo-component)+ "@sha256:" 64hex
# host:          one or more dot-separated DNS hostname labels (each
#                [A-Za-z0-9], with internal "-" allowed but not leading or
#                trailing a label), optionally followed by ":port". Modeled
#                on the domain grammar in the Docker/OCI distribution
#                reference spec (domainNameComponent/domainName in
#                https://github.com/distribution/reference/blob/main/regexp.go).
#                Mixed case is accepted because DNS hostnames are
#                case-insensitive and registries are commonly referenced
#                that way (e.g. "Quay.io"). Consecutive/leading/trailing
#                dots, a label starting/ending in "-", and underscores in
#                the host are all invalid and rejected (e.g. "quay..io",
#                "quay-.io" are not valid registry hostnames). IPv6/other
#                host forms outside this labeled-hostname subset are not
#                supported.
# repo-component: lowercase-alphanumeric runs, separated only by a single
#                "." or "_", a double "__", or a run of one-or-more "-"
#                (this mirrors the Docker/OCI distribution reference spec's
#                path-component grammar). Upper case, a literal "..", and a
#                run of 3+ "_" are all invalid separators and rejected.
# Deliberately does NOT accept a bare "@sha256:..." with no repository, a
# single unqualified segment (implicit docker.io/library, e.g.
# "nginx@sha256:..."), or a floating tag with no digest. Used identically
# for both --allowed-image/--image-allowlist entries and every image
# reference found in the rendered output, so the same string either passes
# or fails both checks.
valid_image_ref() {
  local host_label='[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?'
  [[ "$1" =~ ^${host_label}(\.${host_label})*(:[0-9]+)?(/[a-z0-9]+((\.|_{1,2}|-+)[a-z0-9]+)*)+@sha256:[0-9a-f]{64}$ ]]
}

PINS_FILE=""
OUTPUT_DIR=""
NAMESPACE=""
INSTANCE_NAMESPACE=""
SERVICE_ACCOUNT=""
CONTROLLER_CLUSTER_ROLE=""
IMAGE_PULL_SECRET_NAME=""
ALLOWLIST_FILE=""
declare -a ALLOWED_IMAGES=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --pins) PINS_FILE="$2"; shift 2 ;;
    --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
    --namespace) NAMESPACE="$2"; shift 2 ;;
    --instance-namespace) INSTANCE_NAMESPACE="$2"; shift 2 ;;
    --service-account) SERVICE_ACCOUNT="$2"; shift 2 ;;
    --controller-cluster-role) CONTROLLER_CLUSTER_ROLE="$2"; shift 2 ;;
    --image-pull-secret-name) IMAGE_PULL_SECRET_NAME="$2"; shift 2 ;;
    --allowed-image) ALLOWED_IMAGES+=("$2"); shift 2 ;;
    --image-allowlist) ALLOWLIST_FILE="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1 (see --help)" ;;
  esac
done

for name in PINS_FILE OUTPUT_DIR NAMESPACE INSTANCE_NAMESPACE SERVICE_ACCOUNT IMAGE_PULL_SECRET_NAME; do
  [[ -n "${!name}" ]] || { usage; die "missing required argument for $name"; }
done

require_tool helm
require_tool kubectl
require_tool yq
require_tool jq

[[ -f "$PINS_FILE" ]] || die "pins file not found: $PINS_FILE"

for n in "$NAMESPACE" "$INSTANCE_NAMESPACE"; do
  valid_dns_label "$n" || die "not a valid Kubernetes namespace name (DNS-1123 label, max 63 chars): $n"
done
for n in "$SERVICE_ACCOUNT" "$IMAGE_PULL_SECRET_NAME"; do
  valid_dns_subdomain "$n" || die "not a valid Kubernetes object name (DNS-1123 subdomain, max 253 chars): $n"
done
if [[ -n "$CONTROLLER_CLUSTER_ROLE" ]]; then
  valid_dns_subdomain "$CONTROLLER_CLUSTER_ROLE" || die "invalid --controller-cluster-role (DNS-1123 subdomain, max 253 chars)"
fi

[[ -z "$OUTPUT_DIR" || "$OUTPUT_DIR" != "/" ]] || die "refusing to use / as --output-dir"
[[ -e "$OUTPUT_DIR" ]] && die "output directory already exists, refusing to overwrite: $OUTPUT_DIR"
OUTPUT_PARENT="$(dirname -- "$OUTPUT_DIR")"
[[ -d "$OUTPUT_PARENT" ]] || die "parent of --output-dir does not exist: $OUTPUT_PARENT"

if [[ -n "$ALLOWLIST_FILE" ]]; then
  [[ -f "$ALLOWLIST_FILE" ]] || die "image allowlist file not found: $ALLOWLIST_FILE"
  while IFS= read -r line; do
    [[ -z "$line" || "$line" == \#* ]] && continue
    ALLOWED_IMAGES+=("$line")
  done < "$ALLOWLIST_FILE"
fi

# No bypass: an omitted flag, an empty --image-allowlist file, or an
# allowlist file containing only blanks/comments must all fail closed.
# This tool invents no default approved image or tuple.
[[ ${#ALLOWED_IMAGES[@]} -gt 0 ]] || die "no approved image input supplied: pass --allowed-image (repeatable) and/or a non-empty --image-allowlist"
for a in "${ALLOWED_IMAGES[@]}"; do
  valid_image_ref "$a" || die "approved image is not a valid fully-qualified, digest-pinned reference, refusing to allowlist it: $a"
done

read_pin() {
  local jq_path="$1"
  yq eval -o=json "$PINS_FILE" | jq -r "($jq_path) // empty" 2>/dev/null || true
}

CRDS_CHART_PATH="$(read_pin '.crdsChart.path')"
CRDS_CHART_VERSION="$(read_pin '.crdsChart.version')"
CRDS_CHART_CHECKSUM="$(read_pin '.crdsChart.checksum')"
OPERATOR_CHART_PATH="$(read_pin '.operatorChart.path')"
OPERATOR_CHART_VERSION="$(read_pin '.operatorChart.version')"
OPERATOR_CHART_CHECKSUM="$(read_pin '.operatorChart.checksum')"

for name in CRDS_CHART_PATH CRDS_CHART_VERSION CRDS_CHART_CHECKSUM OPERATOR_CHART_PATH OPERATOR_CHART_VERSION OPERATOR_CHART_CHECKSUM; do
  [[ -n "${!name}" ]] || die "pins file missing required field (resolved empty): $name"
done

PINS_DIR="$(cd "$(dirname -- "$PINS_FILE")" && pwd)"
resolve_chart_path() {
  local p="$1"
  if [[ "$p" = /* ]]; then
    printf '%s\n' "$p"
  else
    printf '%s\n' "$PINS_DIR/$p"
  fi
}
CRDS_CHART_PATH="$(resolve_chart_path "$CRDS_CHART_PATH")"
OPERATOR_CHART_PATH="$(resolve_chart_path "$OPERATOR_CHART_PATH")"

for name in CRDS_CHART_PATH OPERATOR_CHART_PATH; do
  [[ -f "${!name}" ]] || die "chart archive not found: ${!name}"
done

check_checksum() {
  local path="$1" expected="$2" actual
  expected="${expected#sha256:}"
  [[ "$expected" =~ ^[0-9a-f]{64}$ ]] || die "malformed checksum pin for $path: $expected"
  actual="$(sha256_of "$path")"
  [[ "$actual" == "$expected" ]] || die "checksum mismatch for $path: expected $expected, got $actual"
}
check_checksum "$CRDS_CHART_PATH" "$CRDS_CHART_CHECKSUM"
check_checksum "$OPERATOR_CHART_PATH" "$OPERATOR_CHART_CHECKSUM"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gcp-1280-render.XXXXXX")"
trap 'rm -rf "$WORK_DIR"' EXIT
mkdir "$WORK_DIR/rendered" "$WORK_DIR/operator-overlay" "$WORK_DIR/charts"

tar -xzf "$CRDS_CHART_PATH" -C "$WORK_DIR/charts"
tar -xzf "$OPERATOR_CHART_PATH" -C "$WORK_DIR/charts"

check_chart_version() {
  local chart_dir="$1" expected="$2" actual
  actual="$(yq eval '.version' "$chart_dir/Chart.yaml")"
  [[ "$actual" == "$expected" ]] || die "chart version mismatch in $chart_dir: pin says $expected, archive has $actual"
}

# The archive's top-level directory name is the authoritative chart name.
CRDS_CHART_NAME="$(tar -tzf "$CRDS_CHART_PATH" | sed -n '1p' | cut -d/ -f1)"
OPERATOR_CHART_NAME="$(tar -tzf "$OPERATOR_CHART_PATH" | sed -n '1p' | cut -d/ -f1)"
CRDS_CHART_DIR="$WORK_DIR/charts/$CRDS_CHART_NAME"
OPERATOR_CHART_DIR="$WORK_DIR/charts/$OPERATOR_CHART_NAME"
[[ -f "$CRDS_CHART_DIR/Chart.yaml" ]] || die "CRDs archive did not extract a valid chart: $CRDS_CHART_PATH"
[[ -f "$OPERATOR_CHART_DIR/Chart.yaml" ]] || die "operator archive did not extract a valid chart: $OPERATOR_CHART_PATH"

check_chart_version "$CRDS_CHART_DIR" "$CRDS_CHART_VERSION"
check_chart_version "$OPERATOR_CHART_DIR" "$OPERATOR_CHART_VERSION"

# Fail if stripping top-level `status` changed anything beyond that single
# top-level key, including nested CRD schema properties also named `status`.
# Config Sync rejects source objects that declare a top-level status
# (KNV1045); nested schema properties named status are unrelated data and
# must survive untouched.
strip_top_level_status() {
  local file="$1" stripped="$1.stripped"
  awk '
    /^status:/ { skipping = 1; next }
    skipping && (/^---[[:space:]]*$/ || /^[A-Za-z_][A-Za-z0-9_.-]*:/) { skipping = 0 }
    !skipping { print }
  ' "$file" > "$stripped"
  yq eval -o=json -I=0 '.' "$file" | jq -Ss 'map(del(.status))' > "$file.expected.json"
  yq eval -o=json -I=0 '.' "$stripped" | jq -Ss '.' > "$file.actual.json"
  if ! cmp -s "$file.expected.json" "$file.actual.json"; then
    echo "render.sh: unexpected semantic change while removing status from $file" >&2
    exit 1
  fi
  rm -f "$file.expected.json" "$file.actual.json"
  mv "$stripped" "$file"
}

helm template gitops-crds "$CRDS_CHART_DIR" \
  --include-crds > "$WORK_DIR/rendered/crds.yaml"
strip_top_level_status "$WORK_DIR/rendered/crds.yaml"

# Never render a Docker config Secret or any other credential-bearing object
# into the output; imagePullSecrets is a reference to a Secret supplied
# independently by the caller, not a value rendered here.
declare -a CONTROLLER_ROLE_ARGS=()
if [[ -n "$CONTROLLER_CLUSTER_ROLE" ]]; then
  CONTROLLER_ROLE_ARGS=(--set-string "env.CONTROLLER_CLUSTER_ROLE=$CONTROLLER_CLUSTER_ROLE")
fi
helm template gitops-operator "$OPERATOR_CHART_DIR" \
  --namespace "$NAMESPACE" \
  --set pullSecret.create=false \
  --set serviceAccount.name="$SERVICE_ACCOUNT" \
  --set-string env.ENABLE_CONVERSION_WEBHOOK=false \
  --set-string env.ARGOCD_CLUSTER_CONFIG_NAMESPACES="$INSTANCE_NAMESPACE" \
  --set-string "imagePullSecrets[0].name=$IMAGE_PULL_SECRET_NAME" \
  ${CONTROLLER_ROLE_ARGS[@]+"${CONTROLLER_ROLE_ARGS[@]}"} \
  > "$WORK_DIR/operator-overlay/operator-helm.yaml"

# Helm's release namespace is not emitted in most of this chart's manifests;
# the kustomize overlay sets it explicitly before anything applies the YAML.
sed "s/__OPERATOR_NAMESPACE__/$NAMESPACE/g" \
  "$SCRIPT_DIR/overlay/kustomization.yaml.tmpl" > "$WORK_DIR/operator-overlay/kustomization.yaml"
sed "s/__OPERATOR_NAMESPACE__/$NAMESPACE/g" \
  "$SCRIPT_DIR/overlay/namespace.yaml.tmpl" > "$WORK_DIR/operator-overlay/namespace.yaml"
kubectl kustomize "$WORK_DIR/operator-overlay" > "$WORK_DIR/rendered/operator.yaml"
strip_top_level_status "$WORK_DIR/rendered/operator.yaml"

# --- Semantic validation; fail closed on anything unexpected. ---

for file in crds.yaml operator.yaml; do
  yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/$file" |
    jq -se 'all(.[]; has("status") | not)' >/dev/null \
    || die "top-level status survived in $file"
done

# crds.yaml must contain CustomResourceDefinitions only. The CRDs chart's
# templates/ directory (unlike crds/) is not excluded by helm template, so a
# chart that smuggles a Secret or any other object there must be rejected
# before it reaches the output this tool publishes.
crds_only="$(yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/crds.yaml" | jq -s 'all(.[]; .kind == "CustomResourceDefinition")')"
[[ "$crds_only" == "true" ]] || die "crds.yaml renders an object that is not a CustomResourceDefinition; refusing to publish"
crds_count="$(yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/crds.yaml" | jq -s 'length')"
[[ "$crds_count" -gt 0 ]] || die "crds.yaml rendered zero CustomResourceDefinition objects"

# Every object kind in operator.yaml must be one this contract explicitly
# covers; anything else fails closed rather than being published unchecked.
# (Namespace is the overlay-provided namespace object itself.)
unsupported_kinds="$(
  yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/operator.yaml" |
    jq -sr '[.[].kind] | unique[]' |
    grep -vxE 'Namespace|Deployment|ServiceAccount|Service|Role|RoleBinding|ClusterRole|ClusterRoleBinding|Secret|ConfigMap' \
    || true
)"
[[ -z "$unsupported_kinds" ]] || die "operator.yaml renders unsupported object kind(s), refusing to publish: $(tr '\n' ' ' <<<"$unsupported_kinds")"

yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/operator.yaml" |
  jq -se --arg ns "$NAMESPACE" \
    'all(.[] | select(.kind == "Deployment" or .kind == "ServiceAccount" or .kind == "Service" or .kind == "Role" or .kind == "RoleBinding" or .kind == "Secret" or .kind == "ConfigMap"); .metadata.namespace == $ns)' \
    >/dev/null \
  || die "a namespaced object is not scoped to --namespace $NAMESPACE"

yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/operator.yaml" |
  jq -se 'all(.[] | select(.kind == "ClusterRole" or .kind == "ClusterRoleBinding"); .metadata.namespace == null)' \
    >/dev/null \
  || die "a cluster-scoped object unexpectedly declares a namespace"

# No credential-bearing Secret: only an empty, Kubernetes-populated
# service-account-token placeholder is permitted, and only with no data.
yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/operator.yaml" |
  jq -se '
    all(.[] | select(.kind == "Secret");
      .type == "kubernetes.io/service-account-token"
      and ((.data // {}) | length == 0)
      and ((.stringData // {}) | length == 0))
  ' >/dev/null \
  || die "operator.yaml renders a credential-bearing or non-placeholder Secret"

# The requested ServiceAccount must actually be declared, and every
# Deployment must reference it consistently; a token-placeholder Secret, if
# any is rendered, must be annotated for the same ServiceAccount.
sa_declared="$(
  yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/operator.yaml" |
    jq -s --arg sa "$SERVICE_ACCOUNT" 'any(.[] | select(.kind == "ServiceAccount"); .metadata.name == $sa)'
)"
[[ "$sa_declared" == "true" ]] || die "requested --service-account $SERVICE_ACCOUNT is not declared as a ServiceAccount in operator.yaml"

deployment_count="$(yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/operator.yaml" | jq -s '[.[] | select(.kind == "Deployment")] | length')"
[[ "$deployment_count" -gt 0 ]] || die "operator.yaml rendered zero Deployment objects"

deployment_sa_ok="$(
  yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/operator.yaml" |
    jq -s --arg sa "$SERVICE_ACCOUNT" 'all(.[] | select(.kind == "Deployment"); .spec.template.spec.serviceAccountName == $sa)'
)"
[[ "$deployment_sa_ok" == "true" ]] || die "a Deployment does not reference --service-account $SERVICE_ACCOUNT"

token_secret_sa_ok="$(
  yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/operator.yaml" |
    jq -s --arg sa "$SERVICE_ACCOUNT" \
      'all(.[] | select(.kind == "Secret" and .type == "kubernetes.io/service-account-token");
        (.metadata.annotations["kubernetes.io/service-account.name"] // $sa) == $sa)'
)"
[[ "$token_secret_sa_ok" == "true" ]] || die "a service-account-token placeholder Secret is annotated for a different ServiceAccount"

# Verify the manager env settings and pull-secret reference actually landed
# in the rendered output; a chart that ignores the helm --set path for these
# values must not silently pass. These are postconditions of the Deployment
# that owns the manager container specifically (the prototype's container
# name for the operator's main controller) — a sidecar/initContainer, or a
# wholly separate Deployment, carrying the correct values must not be able
# to satisfy them. Resolve that one Deployment (and, within it, that one
# container) first, failing closed if either is missing or ambiguous, then
# check every postcondition against that same Deployment only.
readonly MANAGER_CONTAINER_NAME="manager"
manager_deployments_json="$(
  yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/operator.yaml" |
    jq -sc --arg n "$MANAGER_CONTAINER_NAME" \
      '[.[] | select(.kind == "Deployment") | select([.spec.template.spec.containers[]?.name] | any(. == $n))]'
)"
manager_deployment_count="$(jq 'length' <<<"$manager_deployments_json")"
[[ "$manager_deployment_count" -eq 1 ]] || die "expected exactly one Deployment owning a container named '$MANAGER_CONTAINER_NAME' (found $manager_deployment_count); refusing to guess"
manager_deployment_json="$(jq -c '.[0]' <<<"$manager_deployments_json")"

manager_containers_json="$(jq -c --arg n "$MANAGER_CONTAINER_NAME" '[.spec.template.spec.containers[]? | select(.name == $n)]' <<<"$manager_deployment_json")"
manager_container_count="$(jq 'length' <<<"$manager_containers_json")"
[[ "$manager_container_count" -eq 1 ]] || die "expected exactly one container named '$MANAGER_CONTAINER_NAME' in its Deployment (found $manager_container_count); refusing to guess"
manager_container_json="$(jq -c '.[0]' <<<"$manager_containers_json")"

assert_manager_env_value() {
  local env_name="$1" expected="$2" matches match_count has_literal value
  matches="$(jq -c --arg n "$env_name" '[.env[]? | select(.name == $n)]' <<<"$manager_container_json")"
  match_count="$(jq 'length' <<<"$matches")"
  [[ "$match_count" -eq 1 ]] || die "manager container ('$MANAGER_CONTAINER_NAME') must set exactly one env entry named $env_name (found $match_count, possibly duplicate/conflicting)"
  has_literal="$(jq -r '.[0] | has("value")' <<<"$matches")"
  [[ "$has_literal" == "true" ]] || die "manager container env $env_name does not set a literal value (valueFrom cannot be verified statically)"
  value="$(jq -r '.[0].value' <<<"$matches")"
  [[ "$value" == "$expected" ]] || die "manager container env $env_name set to unexpected value in rendered output: $value (expected $expected)"
}
assert_manager_env_value ENABLE_CONVERSION_WEBHOOK false
assert_manager_env_value ARGOCD_CLUSTER_CONFIG_NAMESPACES "$INSTANCE_NAMESPACE"
if [[ -n "$CONTROLLER_CLUSTER_ROLE" ]]; then
  assert_manager_env_value CONTROLLER_CLUSTER_ROLE "$CONTROLLER_CLUSTER_ROLE"
fi

pull_secret_referenced="$(
  jq -r --arg n "$IMAGE_PULL_SECRET_NAME" \
    '[(.spec.template.spec.imagePullSecrets // [])[]?.name] | any(. == $n)' <<<"$manager_deployment_json"
)"
[[ "$pull_secret_referenced" == "true" ]] || die "the manager Deployment does not reference --image-pull-secret-name $IMAGE_PULL_SECRET_NAME"

# Reject floating/malformed image references and unreviewed image tuples.
# This covers manager and initContainer images directly, plus every env var on
# those containers whose name contains the (case-sensitive, uppercase) string
# IMAGE, the conventional way charts pass operand image overrides (e.g.
# ARGOCD_IMAGE, BACKEND_IMAGE, RELATED_IMAGE_*, GITOPS_CONSOLE_PLUGIN_IMAGE_PF5).
# Every candidate must be a digest-pinned literal AND present in the mandatory
# --allowed-image / --image-allowlist set; such an env set via valueFrom cannot
# be verified statically and fails closed too. Env names without uppercase
# IMAGE are not inspected, and a non-image variable whose name contains IMAGE
# (for example IMAGE_PULL_POLICY) is rejected rather than ignored.
declare -a RENDERED_IMAGES=()
while IFS= read -r image; do
  [[ -n "$image" ]] && RENDERED_IMAGES+=("$image")
done < <(
  yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/operator.yaml" |
    jq -sr '[.[] | (.spec.template.spec.containers // [])[], (.spec.template.spec.initContainers // [])[]] | .[].image' |
    sort -u
)

while IFS=$'\t' read -r env_name has_literal env_value; do
  [[ -z "$env_name" ]] && continue
  if [[ "$has_literal" != "true" ]]; then
    die "image-bearing env var $env_name does not set a literal value (valueFrom or an unset value cannot be verified statically)"
  fi
  RENDERED_IMAGES+=("$env_value")
done < <(
  yq eval -o=json -I=0 '.' "$WORK_DIR/rendered/operator.yaml" |
    jq -sr '
      [.[] | (.spec.template.spec.containers // [])[], (.spec.template.spec.initContainers // [])[]]
      | .[].env[]? | select(.name | test("IMAGE"))
      | [.name, (has("value") | tostring), (.value // "")] | @tsv
    '
)

if [[ ${#RENDERED_IMAGES[@]} -gt 0 ]]; then
  for image in "${RENDERED_IMAGES[@]}"; do
    valid_image_ref "$image" || die "rejecting floating/malformed image reference (not a valid fully-qualified, digest-pinned reference): $image"
    allowed=0
    for a in "${ALLOWED_IMAGES[@]}"; do
      [[ "$image" == "$a" ]] && { allowed=1; break; }
    done
    [[ "$allowed" -eq 1 ]] || die "rejecting unreviewed image tuple (not in --allowed-image/--image-allowlist): $image"
  done
fi

[[ -e "$OUTPUT_DIR" ]] && die "output directory appeared during render: $OUTPUT_DIR"
mv "$WORK_DIR/rendered" "$OUTPUT_DIR"
echo "Rendered $(wc -l < "$OUTPUT_DIR/crds.yaml") lines of CRDs and a credential-free operator manifest in $OUTPUT_DIR"
