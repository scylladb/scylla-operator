{{/*
Expand the name of the chart.
*/}}
{{- define "scylla.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "scylla.fullname" -}}
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
{{- define "scylla.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "scylla.labels" -}}
scylla/cluster: {{ include "scylla.fullname" . }}
app: scylla
app.kubernetes.io/name: scylla
app.kubernetes.io/managed-by: scylla-operator
{{- end }}

{{/*
Selector labels
*/}}
{{- define "scylla.selectorLabels" -}}
app.kubernetes.io/name: {{ include "scylla.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

