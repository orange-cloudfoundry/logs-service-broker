# logs-service-broker

Logs-service-broker is a broker server for logs parsing (with custom parsing patterns given by user or operator) and
forwarding to one or multiple syslog endpoint in RFC 5424 syslog format.
Take care that logs-service-broker will always provide json encoded format to final syslog endpoint(s).

It is for now tied to Cloud Foundry for different types of logs received by this platform.

This is compliant with the spec [open service broker api](https://www.openservicebrokerapi.org/) for
[syslog drain](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md#log-drain).

## How to deploy

On Cloud Foundry, a service should **not** be deployed from this source code, but it must use the boshrelease related to it
which can be found here: https://github.com/orange-cloudfoundry/logservice-boshrelease/

1. Clone the repo
2. Go build the repo directly this will give you a logs-service-broker runnable server
3. Create a `config.yml` file to set your configuration. Configuration is explained in the [configuration section](#configuration)

## Configuration

Config format is explained in [config-sample.yml](./config-sample.yml).

* `[mandatory]` tag means current key is mandatory
* types must follow those given as example

### .syslog_addresses configuration

**Note**: Default grok patterns can be found at [parser/patterns.go](./parser/patterns.go) and [vendor/github.com/ArthurHlt/grok/patterns.go](./vendor/github.com/ArthurHlt/grok/patterns.go).

#### .syslog_addresses.tags templating

Tags can be dynamically be formatted by using golang templating:
```yaml
tags:
  my-tag: "{{ .App }}-my-tag"
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
  this `{"foo": {"exists": ["my-value"]}` can be done with `ret "foo.exists.0"`

**tips**: on `ret` function you can use special key `first` and `last` on a slice for respectively the first value of a slice or the last one.

### forwarder.parsing_key

Some of the key/value pairs have special effect; those pairs defined will be used as parsing value until there is nothing to parse anymore, we call them parsing keys.

By default, parsing keys are:
- `@message`
- `@raw`
- `text`

But as an operator you can provide more.

Here the syntax to use for adding more parsing keys:
```yaml
# Name is the key name to add for parsing, you can chose sub key with this format:
# inline.key.with.dot.separator
# note that if you want to navigate in an array you can use index number in the format and/or last and first keyword to
# get the last or the first element
name: <string>
# If set to true, this will remove this key from final result and just let new parsed value from it
[ hide: <boolean> ]
```

Example, you have the structure parsed as followed:
```json
{
  "foo": {
    "bar": [{
      "elem1" : "text need to be parsed with current patterns"
    }]
  },
  "titi": "toto"
}
```

You can define a parsing key as follows:

```yaml
parsing_keys:
- name: foo.bar.0.elem1
  hide: true
```

you will receive this final json:
```json
{
  "@message": "text need to be parsed with current patterns",
  "titi": "toto"
}
```

## How to use as a user

As documentation is tied to the configuration given by the operator. We will not provide full doc directly here.

User doc can be found when you have deployed logservice at http://<your logservice url>/docs.

For now, subset of user doc can be found here: [user-doc.md](/user-doc.md)

## Prometheus metrics

The broker provide metrics in prometheus format on the endpoint: https://my-logservice.com/metrics .

You can found dashboard for grafana here: https://github.com/orange-cloudfoundry/logservice-boshrelease/blob/master/jobs/logservice_dashboards/templates/logservice_overview.json
And also alerts for it here: https://github.com/orange-cloudfoundry/logservice-boshrelease/blob/master/jobs/logservice_alerts/templates/logservice.alerts.yml

## Architecture in a Cloud Foundry context

[![archi](/docs/archi.png)](/docs/archi.png)

<!-- Local Variables: -->
<!-- ispell-local-dictionary: "american" -->
<!-- End: -->
