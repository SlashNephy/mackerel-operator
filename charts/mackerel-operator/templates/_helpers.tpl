{{/*
Expand the chart name.
*/}}
{{- define "mackerel-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "mackerel-operator.fullname" -}}
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

{{/*
Create chart label.
*/}}
{{- define "mackerel-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "mackerel-operator.labels" -}}
helm.sh/chart: {{ include "mackerel-operator.chart" . }}
{{ include "mackerel-operator.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels.
*/}}
{{- define "mackerel-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mackerel-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Create the service account name.
*/}}
{{- define "mackerel-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "mackerel-operator.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Render a value from values.yaml as a CEL literal. Every element is wrapped in
dyn() because CEL rejects map and list literals whose elements differ in type.
*/}}
{{- define "mackerel-operator.celLiteral" -}}
{{- if kindIs "map" . -}}
{{- $entries := list -}}
{{- range $key, $value := . -}}
{{- $entries = append $entries (printf "%s: dyn(%s)" (toJson $key) (include "mackerel-operator.celLiteral" $value)) -}}
{{- end -}}
{{- printf "{%s}" (join ", " $entries) -}}
{{- else if kindIs "slice" . -}}
{{- $items := list -}}
{{- range . -}}
{{- $items = append $items (printf "dyn(%s)" (include "mackerel-operator.celLiteral" .)) -}}
{{- end -}}
{{- printf "[%s]" (join ", " $items) -}}
{{- else -}}
{{- toJson . -}}
{{- end -}}
{{- end -}}
