You are using plan [{{ title .InstanceParam.SyslogName }}](#{{ slug .InstanceParam.SyslogName}}){{ with .InstanceParam.DrainType}} 
and you are draining logs of type `{{ . }}`{{end}}{{ with .InstanceParam.UseTls }} with TLS activated{{ end }}.

You have actually **{{ len .LogMetadatas }}** apps bound to this service.

Your service is actually the {{ .InstanceParam.Revision }} revision.

{{- with .InstanceParam.Tags }}
### Your current tags
{{- range . }}
- **{{ .Key }}**: `{{ safe .Value }}`
{{- end }}
{{ end -}}

{{- with .InstanceParam.Patterns }}
### Your current pattern
{{- range . }}
- `{{ safe .Pattern }}`
{{- end }}
{{ end -}}
