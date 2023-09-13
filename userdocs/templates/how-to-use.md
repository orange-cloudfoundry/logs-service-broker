## Create service
Simply run create-service command on cf cli and bind it to an app:
```bash
$ cf create-service logs <plan> my-log-service
$ cf bind-service <my-app> my-log-service
```

You can also set parameter on your service to define patterns and tags:
```bash
$ cf create-service logs <plan> my-log-service -c '{"tags": {"my-tag": "bar"}, "patterns": ["%{GREEDYDATA":my-data}"]}'
```

**Note**:
- you can found plan in [available plans section](#available-plans)
- More about tags formatting in [tags formatting section](#tags-formatting)
- More about patterns and grok available patterns in [patterns formatting section](#patterns-formatting)

## Update service

Run update-service command on cf cli and rebind your bindings:
```bash
$ cf update-service my-log-service -c '{"tags": {"my-tag": "bar"}, "patterns": ["%{GREEDYDATA:my-data}"]}'
$ cf unbind-service <my-app> my-log-service
$ cf bind-service <my-app> my-log-service
```

**Note**: Unbinding is necessary for logservice to consider your new changes

## Available parameters

When creating or updating a service, the following parameters can be passed:
- `tags` (*Map key value*): Define your tags (see tags formatting in [tags formatting section](#tags-formatting))
- `patterns` (*Slice of string*): Define your patter (see patterns and grok available patterns in [patterns formatting section](#patterns-formatting))
{{ if not .Config.Broker.ForceEmptyDrainType }}- `drain_type` (*can be `logs` (similar to empty), `metrics` or `all`*): Allow metrics or both logs and metrics to be sent in logservice.
(**Warning** Metrics should be use when you have not prometheus, a lot of dashboards are already available on it){{ end }}


## Tags formatting

Tags can be dynamically be formatted by using golang templating:
```json
{
  "tags": {
    "my-tag": "{{"{{"}} .App {{"}}"}}-my-tag"
  }
}
```

This example show how to suffix your tag `my-tag` by the app name for current log.

You have access to this data:
- `Org`: Org name in current log
- `OrgID`: Org id in current log
- `Space`: Space name in current log
- `SpaceID`: Space id in current log
- `App`: App name in current log
- `AppID`: App id in current log
- `Logdata`: Final logs parsed as a `map[string]interface{}` (use `ret` function for easy exploring)

In addition, you can use those functions for helping you:
- `split <param> <delimiter>`: Split string by a delimiter to get a slice  
- `join <param> <delimiter>`: Make string from a slice collapse by delimiter
- `trimSuffix <param> <suffix>`: Remove suffix from param
- `trimPrefix <param> <prefix>`: Remove prefix from param
- `hasPrefix <param> <prefix>`: Check if prefix exists in param
- `hasSuffix <param> <prefix>`: Check if suffix exists in param
- `ret access.to.value.from.key`: Get the value of a key in a map by exploring it in dot format, e.g: 
this `{"foo": {"exists": ["my-value"]}` can be accessed with `ret "foo.exists.0"`

**tips**: on `ret` function you can use special key `first` and `last` on a slice for respectively the first value of a slice or the last one.

## Patterns formatting

Patterns use grok format which is simple to use. Its goal is to parse logs as you have asked and place it in structured data.

for example:
- `"%{.*:my-data}"` will place all log message like this:

```json
{
  "my-data": "my log message"
}
```

- `"%{GREEDYDATA:my-data}"` will do the same because `GREEDYDATA` is pre-provisioned patterns by logservice which is equivalent to `.*`.
- `"%{GREEDYDATA:[my-data][value]}"` will place all log message like this:

```json
{
  "my-data": {
    "value": "my log message"
  }
}
```

You can see all pre-provisioned patterns [here](#pre-provisioned-patterns).

### Special key/value pairs

Some of the key/value pairs have special effect, those pairs defined will be used as parsing value until there is nothing to parse anymore.

These key/value pairs are:
- `@message`
- `@raw`
- `text`
{{- with .Config.Forwarder.ParsingKeys }}
{{- range . }}
- `{{ .Name }}`{{ with .Hide }} (*this key will be hidden from parsed log*){{end}}
{{- end }}
{{ end -}}

It is always good to set one of those keys in your pattern.

Example:

I defined these patterns:
```json
{
  "patterns": [
    "my message %{GREEDYDATA:@message}",
    "%{final text:@final}"
  ]
}
```

With this log message:
```bash
my message some data final text
```

Will be parsed as follows:
```json
{
  "@message": "some data final text",
  "@final": "final text"
}
```

