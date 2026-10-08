#!/usr/bin/env bash
set -euo pipefail

tests_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
control_dir="$(cd -- "$tests_dir/.." && pwd)"
normalizer="$control_dir/scripts/normalize-gcloud-snapshot.jq"

backend_json='{"name":"synthetic-backend","loadBalancingScheme":"EXTERNAL_MANAGED","securityPolicy":"https://compute.example/projects/synthetic/global/securityPolicies/sample","usedBy":[{"reference":"https://compute.example/projects/synthetic/global/urlMaps/synthetic-map"}],"logConfig":{"enable":true,"sampleRate":1.0}}'
policy_json='[{"name":"sample","selfLink":"https://compute.example/projects/synthetic/global/securityPolicies/sample","rules":[{"priority":1000,"action":"deny(403)","preview":false,"match":{"expr":{"expression":"evaluatePreconfiguredWaf('\''xss-v33-stable'\'')"}}},{"priority":2147483647,"action":"allow","match":{"versionedExpr":"SRC_IPS_V1"}}]}]'
frontend_evidence='{"urlMaps":[{"selfLink":"https://compute.example/projects/synthetic/global/urlMaps/synthetic-map"}],"proxies":[{"prototypeProtocol":"HTTPS","selfLink":"https://compute.example/projects/synthetic/global/targetHttpsProxies/synthetic-proxy","urlMap":"https://compute.example/projects/synthetic/global/urlMaps/synthetic-map"}],"forwardingRules":[{"target":"https://compute.example/projects/synthetic/global/targetHttpsProxies/synthetic-proxy","loadBalancingScheme":"EXTERNAL_MANAGED"}]}'

assessment="$(
	jq -n \
		--arg target_id "synthetic-backend" \
		--argjson backend "$backend_json" \
		--argjson policy "$policy_json" \
		--argjson frontend_evidence "$frontend_evidence" \
		-f "$normalizer"
)"

jq -e '
	.target.id == "synthetic-backend"
	and .target.internetFacing == true
	and .target.loadBalancingScheme == "EXTERNAL_MANAGED"
	and .backendService.securityPolicy == .securityPolicy.selfLink
	and .backendService.logConfig.enable == true
	and .backendService.logConfig.sampleRate == 1
	and (.securityPolicy.rules | length) == 2
' <<<"$assessment" >/dev/null

internal_backend="$(jq -c '.loadBalancingScheme = "INTERNAL_MANAGED"' <<<"$backend_json")"
internal_evidence="$(jq -c '.forwardingRules[0].loadBalancingScheme = "INTERNAL_MANAGED"' <<<"$frontend_evidence")"
internal_assessment="$(
	jq -n \
		--arg target_id "synthetic-backend" \
		--argjson backend "$internal_backend" \
		--argjson policy "$policy_json" \
		--argjson frontend_evidence "$internal_evidence" \
		-f "$normalizer"
)"
jq -e '.target.internetFacing == false' <<<"$internal_assessment" >/dev/null

no_frontend_assessment="$(
	jq -n \
		--arg target_id "synthetic-backend" \
		--argjson backend "$backend_json" \
		--argjson policy "$policy_json" \
		--argjson frontend_evidence '{"urlMaps":[{"selfLink":"https://compute.example/projects/synthetic/global/urlMaps/synthetic-map"}],"proxies":[],"forwardingRules":[]}' \
		-f "$normalizer"
)"
jq -e '.target.loadBalancingScheme == "EXTERNAL_MANAGED" and .target.internetFacing == false' <<<"$no_frontend_assessment" >/dev/null

unrelated_map_evidence="$(jq -c '.urlMaps[0].selfLink = "https://compute.example/projects/synthetic/global/urlMaps/unrelated-map"' <<<"$frontend_evidence")"
unrelated_map_assessment="$(
	jq -n \
		--arg target_id "synthetic-backend" \
		--argjson backend "$backend_json" \
		--argjson policy "$policy_json" \
		--argjson frontend_evidence "$unrelated_map_evidence" \
		-f "$normalizer"
)"
jq -e '.target.internetFacing == false' <<<"$unrelated_map_assessment" >/dev/null

printf 'gcloud normalizer tests passed (synthetic input only)\n'
