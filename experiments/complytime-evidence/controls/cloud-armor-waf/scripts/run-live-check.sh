#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
project_id=""
backend_service=""
security_policy=""

usage() {
	cat <<'EOF'
Read-only Cloud Armor assessment using live Google Cloud configuration.

Usage:
  bash scripts/run-live-check.sh \
    --project PROJECT_ID \
    --backend-service BACKEND_SERVICE_NAME \
    --security-policy SECURITY_POLICY_NAME

This prototype currently supports global backend services and global
Cloud Armor security policies. It calls read-only gcloud describe/list commands.
EOF
}

while (($#)); do
	case "$1" in
		--project)
			(($# >= 2)) || { usage >&2; exit 2; }
			project_id="$2"
			shift 2
			;;
		--backend-service)
			(($# >= 2)) || { usage >&2; exit 2; }
			backend_service="$2"
			shift 2
			;;
		--security-policy)
			(($# >= 2)) || { usage >&2; exit 2; }
			security_policy="$2"
			shift 2
			;;
		--help|-h)
			usage
			exit 0
			;;
		*)
			printf 'unknown argument: %s\n' "$1" >&2
			usage >&2
			exit 2
			;;
	esac
done

if [[ -z "$project_id" || -z "$backend_service" || -z "$security_policy" ]]; then
	usage >&2
	exit 2
fi

for tool in gcloud jq; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		printf 'required command not found: %s\n' "$tool" >&2
		exit 2
	fi
done

scan_runner="$script_dir/run-complytime-scan.sh"
if [[ ! -x "$scan_runner" ]]; then
	printf 'the shared ComplyTime scan runner is missing from the WAF control scripts directory\n' >&2
	exit 2
fi

tmp_dir="$(mktemp -d)"
trap 'rm -f "$tmp_dir/backend-service.json" "$tmp_dir/security-policy.json" "$tmp_dir/assessment.json" "$tmp_dir/complytime-output.txt" "$tmp_dir/url-maps.jsonl" "$tmp_dir/proxies.jsonl" "$tmp_dir/forwarding-rules.jsonl" "$tmp_dir/url-map.json" "$tmp_dir/proxy-list.json" "$tmp_dir/forwarding-rules.json" "$tmp_dir/frontend-evidence.json"; rmdir "$tmp_dir"' EXIT

if ! gcloud compute backend-services describe "$backend_service" \
	--global \
	--project="$project_id" \
	--format=json \
	--quiet >"$tmp_dir/backend-service.json" 2>/dev/null; then
	printf 'could not read the named global backend service; check gcloud login, project access, and resource name\n' >&2
	exit 2
fi

if ! gcloud compute security-policies describe "$security_policy" \
	--global \
	--project="$project_id" \
	--format=json \
	--quiet >"$tmp_dir/security-policy.json" 2>/dev/null; then
	printf 'could not read the named global Cloud Armor policy; check project access and resource name\n' >&2
	exit 2
fi

for evidence_file in url-maps.jsonl proxies.jsonl forwarding-rules.jsonl url-map.json proxy-list.json forwarding-rules.json frontend-evidence.json; do
	: >"$tmp_dir/$evidence_file"
done

map_refs="$(jq -r '[.usedBy[]?.reference | select(type == "string" and test("/global/urlMaps/[^/]+$"))] | unique | .[]' "$tmp_dir/backend-service.json")"
while IFS= read -r map_ref; do
	[[ -z "$map_ref" ]] && continue
	map_name="${map_ref##*/}"
	if ! gcloud compute url-maps describe "$map_name" \
		--global \
		--project="$project_id" \
		--format=json \
		--quiet >"$tmp_dir/url-map.json" 2>/dev/null; then
		printf 'could not read a URL map associated with the named backend service\n' >&2
		exit 2
	fi
	if ! jq -e --arg map_ref "$map_ref" '.selfLink == $map_ref' "$tmp_dir/url-map.json" >/dev/null; then
		printf 'gcloud returned an unexpected URL map reference\n' >&2
		exit 2
	fi
	jq -c . "$tmp_dir/url-map.json" >>"$tmp_dir/url-maps.jsonl"
	map_self="$(jq -er '.selfLink' "$tmp_dir/url-map.json")"

	for protocol in HTTP HTTPS; do
		case "$protocol" in
			HTTP) proxy_collection="target-http-proxies" ;;
			HTTPS) proxy_collection="target-https-proxies" ;;
		esac
		if ! gcloud compute "$proxy_collection" list \
			--global \
			--project="$project_id" \
			--filter="urlMap=$map_self" \
			--format=json >"$tmp_dir/proxy-list.json" 2>/dev/null; then
			printf 'could not list global HTTP(S) proxies for an associated URL map\n' >&2
			exit 2
		fi
		if ! jq -c --arg map_self "$map_self" --arg protocol "$protocol" \
			'.[] | select(.urlMap == $map_self) | . + {prototypeProtocol: $protocol}' \
			"$tmp_dir/proxy-list.json" >>"$tmp_dir/proxies.jsonl"; then
			printf 'could not normalize global HTTP(S) proxy results\n' >&2
			exit 2
		fi

		while IFS= read -r proxy_self; do
			[[ -z "$proxy_self" ]] && continue
			if ! gcloud compute forwarding-rules list \
				--global \
				--project="$project_id" \
				--filter="target=$proxy_self" \
				--format=json >"$tmp_dir/forwarding-rules.json" 2>/dev/null; then
				printf 'could not list global forwarding rules for an associated HTTP(S) proxy\n' >&2
				exit 2
			fi
			if ! jq -c --arg proxy_self "$proxy_self" \
				'.[] | select(.target == $proxy_self)' \
				"$tmp_dir/forwarding-rules.json" >>"$tmp_dir/forwarding-rules.jsonl"; then
				printf 'could not normalize global forwarding-rule results\n' >&2
				exit 2
			fi
		done < <(jq -r '.[].selfLink' "$tmp_dir/proxy-list.json")
	done
done <<<"$map_refs"

if ! jq -n \
	--slurpfile url_maps "$tmp_dir/url-maps.jsonl" \
	--slurpfile proxies "$tmp_dir/proxies.jsonl" \
	--slurpfile forwarding_rules "$tmp_dir/forwarding-rules.jsonl" \
	'{urlMaps: $url_maps, proxies: $proxies, forwardingRules: $forwarding_rules}' \
	>"$tmp_dir/frontend-evidence.json"; then
	printf 'could not assemble the load-balancer frontend evidence\n' >&2
	exit 2
fi

if ! jq -n \
	--arg target_id "$backend_service" \
	--slurpfile backend "$tmp_dir/backend-service.json" \
	--slurpfile policy "$tmp_dir/security-policy.json" \
	--slurpfile frontend_evidence "$tmp_dir/frontend-evidence.json" \
	-f "$script_dir/normalize-gcloud-snapshot.jq" \
	>"$tmp_dir/assessment.json"; then
	printf 'could not normalize gcloud output into the assessment input\n' >&2
	exit 2
fi

if ! bash "$scan_runner" \
	--input-file "$tmp_dir/assessment.json" \
	--target-id "$backend_service" \
	--expected-result Any >"$tmp_dir/complytime-output.txt" 2>&1; then
	printf 'ComplyTime could not complete the WAF scan; inspect the ignored local .complytime run diagnostics\n' >&2
	exit 2
fi

result="$(sed -n 's/^EvaluationLog requirement result: //p' "$tmp_dir/complytime-output.txt" | tail -n 1)"
case "$result" in
	Passed|Failed) ;;
	*)
		printf 'ComplyTime completed without a recognized WAF EvaluationLog result; inspect the ignored local .complytime run diagnostics\n' >&2
		exit 2
		;;
esac

jq -cn \
	--arg requirement_id "EXAMPLE-WAF-POC-1" \
	--arg result "$result" \
	--slurpfile assessment "$tmp_dir/assessment.json" \
	'($assessment[0]) as $input
	| {
		source: "live-gcloud",
		requirement_id: $requirement_id,
		result: $result,
		observations: {
			load_balancing_scheme: $input.target.loadBalancingScheme,
			internet_facing: $input.target.internetFacing,
			external_frontends: $input.target.externalFrontends,
			security_policy_attached: (
				$input.backendService.securityPolicy == $input.securityPolicy.selfLink
				and $input.backendService.securityPolicy != ""
			),
			active_cloud_armor_action_count: [
				$input.securityPolicy.rules[]?
				| select((.preview // false) == false)
				| select((.action | test("^(deny\\(|throttle$|rate_based_ban$)")))
			] | length,
			request_logging_enabled: $input.backendService.logConfig.enable,
			request_log_sample_rate: $input.backendService.logConfig.sampleRate
		}
	}'
