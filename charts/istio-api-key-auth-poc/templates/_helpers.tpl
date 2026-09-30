{{- define "istio-api-key-auth-poc.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "istio-api-key-auth-poc.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "istio-api-key-auth-poc.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "istio-api-key-auth-poc.labels" -}}
helm.sh/chart: {{ include "istio-api-key-auth-poc.chart" . }}
{{ include "istio-api-key-auth-poc.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "istio-api-key-auth-poc.selectorLabels" -}}
app.kubernetes.io/name: {{ include "istio-api-key-auth-poc.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- /*
The Secret the Deployment reads DATABASE_URL from. When the bundled
Postgres is enabled, this is the chart-managed Secret in secret.yaml;
otherwise it's a pre-existing Secret the operator created (ADR-0005) —
same default name either way, so the app's wiring never changes shape.
*/ -}}
{{- define "istio-api-key-auth-poc.databaseSecretName" -}}
{{- .Values.externalDatabase.existingSecret | default (printf "%s-db" (include "istio-api-key-auth-poc.fullname" .)) -}}
{{- end -}}

{{- define "istio-api-key-auth-poc.databaseSecretKey" -}}
{{- .Values.externalDatabase.existingSecretKey | default "DATABASE_URL" -}}
{{- end -}}

{{- /*
Mirrors the Bitnami postgresql subchart's own common.names.fullname logic
(un-aliased, no nameOverride): "<release>-postgresql", or just "<release>"
if the release name already contains "postgresql". Fails loudly rather than
silently pointing DATABASE_URL at a Service that doesn't exist if the
subchart's naming is overridden in a way we can't mirror here.
*/ -}}
{{- define "istio-api-key-auth-poc.postgresqlHost" -}}
{{- if or (hasKey .Values.postgresql "fullnameOverride") (hasKey .Values.postgresql "nameOverride") -}}
{{- if or .Values.postgresql.fullnameOverride .Values.postgresql.nameOverride -}}
{{- fail "postgresql.fullnameOverride/nameOverride are not supported: this chart derives the bundled Postgres host itself and assumes the subchart's default naming" -}}
{{- end -}}
{{- end -}}
{{- $name := "postgresql" -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
