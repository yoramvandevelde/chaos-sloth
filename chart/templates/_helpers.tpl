{{- define "chaos-sloth.name" -}}
{{- .Chart.Name }}
{{- end }}

{{- define "chaos-sloth.fullname" -}}
{{- if .Release.Name | eq "chaos-sloth" }}
{{- .Chart.Name }}
{{- else }}
{{- printf "%s-%s" .Release.Name .Chart.Name }}
{{- end }}
{{- end }}

{{- define "chaos-sloth.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{ include "chaos-sloth.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "chaos-sloth.selectorLabels" -}}
app.kubernetes.io/name: {{ include "chaos-sloth.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "chaos-sloth.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "chaos-sloth.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "chaos-sloth.secretName" -}}
{{- if .Values.proxmox.existingSecret }}
{{- .Values.proxmox.existingSecret }}
{{- else }}
{{- include "chaos-sloth.fullname" . }}
{{- end }}
{{- end }}
