{{/*
Expand the name of the chart.
*/}}
{{- define "scylla-manager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "scylla-manager.fullname" -}}
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

{{- define "scylla-manager.controllerName" -}}
{{- printf "%s-controller" ( include "scylla-manager.fullname" . ) }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "scylla-manager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "scylla-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "scylla-manager.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "scylla-manager.controllerSelectorLabels" -}}
app.kubernetes.io/name: {{ include "scylla-manager.name" . }}-controller
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}


{{/*
Common labels
*/}}
{{- define "scylla-manager.labels" -}}
{{ include "scylla-manager.selectorLabels" . }}
{{- end }}

{{- define "scylla-manager.controllerLabels" -}}
{{ include "scylla-manager.controllerSelectorLabels" . }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "scylla-manager.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "scylla-manager.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use by Scylla Manager Controller
*/}}
{{- define "scylla-manager.controllerServiceAccountName" -}}
{{- if .Values.controllerServiceAccount.create }}
{{- default ( printf "%s-controller" ( include "scylla-manager.fullname" . ) ) .Values.controllerServiceAccount.name }}
{{- else }}
{{- default "default" .Values.controllerServiceAccount.name }}
{{- end }}
{{- end }}

{{/*
Workaround of https://github.com/helm/helm/issues/3920
Call a template from the context of a subchart.

Usage:
  {{ include "call-nested" (list . "<subchart_name>" "<subchart_template_name>") }}
*/}}
{{- define "call-nested" }}
{{- $dot := index . 0 }}
{{- $subchart := index . 1 | splitList "." }}
{{- $template := index . 2 }}
{{- $values := $dot.Values }}
{{- range $subchart }}
{{- $values = index $values . }}
{{- end }}
{{- include $template (dict "Chart" (dict "Name" (last $subchart)) "Values" $values "Release" $dot.Release "Capabilities" $dot.Capabilities) }}
{{- end }}
