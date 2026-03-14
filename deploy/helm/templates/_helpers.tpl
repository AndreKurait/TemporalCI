{{/*
Common labels
*/}}
{{- define "temporalci.labels" -}}
app.kubernetes.io/name: temporalci
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "temporalci.selectorLabels" -}}
app.kubernetes.io/name: temporalci
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
