{{- define "gitops-operator.serviceAccountName" -}}
{{- required "serviceAccount.name is required" .Values.serviceAccount.name -}}
{{- end -}}

{{/*
GCP-1286 scoped identity: the only bind the manager keeps. It is limited by
resourceNames to the ClusterRoles the bundle pre-creates for the Argo CD
controller and server. Without resourceNames, bind would let the manager bind
any ClusterRole, which is cluster-admin-equivalent.
*/}}
{{- define "gitops-operator.scopedBindRule" -}}
{{- $controller := required "scopedRbac.controllerClusterRole is required when scopedRbac.enabled" .Values.scopedRbac.controllerClusterRole }}
{{- $server := required "scopedRbac.serverClusterRole is required when scopedRbac.enabled" .Values.scopedRbac.serverClusterRole }}
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["clusterroles"]
  verbs: ["bind"]
  resourceNames: [{{ $controller | quote }}, {{ $server | quote }}]
{{- end -}}
