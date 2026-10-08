package cloudarmor

// Normalized prototype input, not a native Cloud Armor API schema.
target: {
	id:             string
	internetFacing: bool
	loadBalancingScheme?: string
	externalFrontends?: [...{
		protocol:             "HTTP" | "HTTPS"
		loadBalancingScheme: string
	}]
	...
}

backendService: {
	name:           string
	securityPolicy: string
	logConfig: {
		enable:     bool
		sampleRate: number & >=0 & <=1
	}
	...
}

securityPolicy: {
	name:     string
	selfLink: string
	rules: [...{
		priority: int
		action:   string
		preview?: bool
		match: {
			expr?: {
				expression?: string
				...
			}
			versionedExpr?: string
			config?: {...}
			...
		}
		...
	}]
	...
}
