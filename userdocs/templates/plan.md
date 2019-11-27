## {{ title .Name }}

{{ safe .Description }}

{{ safe (join .Bullets "\n") }}

{{ with .DefaultDrainType }}
**Default log drain type**: `{{ . }}`
{{ end }}

### Forwarding logs to target(s)
{{- range .URLs }}
- {{ . }}
{{ end }}

{{- with .Tags }}
### Default tags
{{- range $key, $value := . }}
- **{{ $key }}**: `{{ safe $value }}`
{{- end }}
{{ end -}}

{{- with .Patterns }}
### Default patterns:
{{- range . }}
- `{{ safe . }}`
{{- end }}
{{ end -}}
