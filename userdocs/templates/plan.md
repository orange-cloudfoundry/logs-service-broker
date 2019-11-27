## {{ title .Name }}

{{ .Description }}

{{ join .Bullets "\n" }}

{{- with .DefaultDrainType }}
**Default log drain type**: `{{ . }}`
{{ end }}

### Forwarding logs to target(s)
{{- range .URLs }}
- {{ . }}
{{ end }}

{{- with .Tags }}
### Default tags
{{- range $key, $value := . }}
- **{{ $key }}**: `{{ $value }}`
{{ end }}
{{ end -}}

{{- with .Patterns }}
### Default patterns:
{{- range . }}
- `{{ . }}`
{{ end }}
{{ end -}}
