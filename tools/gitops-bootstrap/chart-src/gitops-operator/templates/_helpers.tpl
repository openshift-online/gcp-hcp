{{- define "gitops-operator.serviceAccountName" -}}
{{- required "serviceAccount.name is required" .Values.serviceAccount.name -}}
{{- end -}}
