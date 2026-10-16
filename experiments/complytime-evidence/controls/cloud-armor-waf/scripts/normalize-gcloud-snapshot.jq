def one_resource:
	if type == "array" then
		if length == 1 then .[0] else error("expected exactly one resource in gcloud response") end
	else
		.
	end;

($backend | one_resource | one_resource) as $backend_service
| ($policy | one_resource | one_resource) as $security_policy
| ($frontend_evidence | one_resource) as $frontend
| (
	[
		$frontend.proxies[]? as $proxy
		| select(any($frontend.urlMaps[]?; .selfLink == $proxy.urlMap))
		| $frontend.forwardingRules[]? as $rule
		| select($rule.target == $proxy.selfLink)
		| select($rule.loadBalancingScheme == "EXTERNAL" or $rule.loadBalancingScheme == "EXTERNAL_MANAGED")
		| {
			protocol: $proxy.prototypeProtocol,
			loadBalancingScheme: $rule.loadBalancingScheme
		}
	] | unique_by([.protocol, .loadBalancingScheme])
) as $external_frontends
| {
	target: {
		id: $target_id,
		loadBalancingScheme: ($backend_service.loadBalancingScheme // "UNKNOWN"),
		internetFacing: ($external_frontends | length > 0),
		externalFrontends: $external_frontends
	},
	backendService: {
		name: ($backend_service.name // $target_id),
		securityPolicy: ($backend_service.securityPolicy // ""),
		logConfig: {
			enable: ($backend_service.logConfig.enable // false),
			sampleRate: ($backend_service.logConfig.sampleRate // 0)
		}
	},
	securityPolicy: {
		name: ($security_policy.name // ""),
		selfLink: ($security_policy.selfLink // ""),
		rules: ($security_policy.rules // [])
	}
}
