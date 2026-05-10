{{/*
Expand the name of the chart.
*/}}
{{- define "kube-ops-view.name" -}}
{{- default .Chart.Name .Values.app.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kube-ops-view.fullname" -}}
{{- if .Values.app.fullnameOverride }}
{{- .Values.app.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.app.nameOverride }}
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
{{- define "kube-ops-view.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kube-ops-view.labels" -}}
helm.sh/chart: {{ include "kube-ops-view.chart" . }}
{{ include "kube-ops-view.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: kube-ops-view
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kube-ops-view.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kube-ops-view.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: frontend
{{- end }}

{{/*
Redis labels
*/}}
{{- define "kube-ops-view.redis.labels" -}}
helm.sh/chart: {{ include "kube-ops-view.chart" . }}
{{ include "kube-ops-view.redis.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: kube-ops-view
{{- end }}

{{/*
Redis selector labels
*/}}
{{- define "kube-ops-view.redis.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kube-ops-view.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: redis
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kube-ops-view.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kube-ops-view.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the image name
*/}}
{{- define "kube-ops-view.image" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.image.registry -}}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}
{{- else }}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}
{{- end }}
{{- end }}

{{/*
Create the Redis image name
*/}}
{{- define "kube-ops-view.redis.image" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.redis.image.registry -}}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry .Values.redis.image.repository .Values.redis.image.tag }}
{{- else }}
{{- printf "%s:%s" .Values.redis.image.repository .Values.redis.image.tag }}
{{- end }}
{{- end }}

{{/*
Create Redis URL
*/}}
{{- define "kube-ops-view.redis.url" -}}
{{- if .Values.redis.enabled }}
{{- printf "redis://%s-redis:6379" (include "kube-ops-view.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Create config checksum annotation
*/}}
{{- define "kube-ops-view.configChecksum" -}}
checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
{{- end }}

{{/*
Create secret checksum annotation
*/}}
{{- define "kube-ops-view.secretChecksum" -}}
checksum/secret: {{ include (print $.Template.BasePath "/secret.yaml") . | sha256sum }}
{{- end }}