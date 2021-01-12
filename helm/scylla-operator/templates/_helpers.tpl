{{/*
Expand the name of the chart.
*/}}
{{- define "scylla-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "scylla-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "scylla-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "scylla-operator.labels" -}}
helm.sh/chart: {{ include "scylla-operator.chart" . }}
{{ include "scylla-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "scylla-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "scylla-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "scylla-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "scylla-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the webhook cerfiticate
*/}}
{{- define "scylla-operator.certificateName" -}}
{{- if .Values.webhook.createSelfSignedCertificate }}
{{- printf "%s-%s" .Chart.Name "selfsigned-certificate" }}
{{- end }}
{{- end }}


{{/*
Create the name of the webhook cerfiticate secret
*/}}
{{- define "scylla-operator.certificateSecretName" -}}
{{- if .Values.webhook.createSelfSignedCertificate }}
{{- default "scylla-operator-serving-cert" .Values.webhook.certificateSecretName }}
{{- else }}
{{- .Values.webhook.certificateSecretName }}
{{- end }}
{{- end }}
