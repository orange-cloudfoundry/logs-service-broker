# logs-service-broker

Logs-service-broker is a broker server for logs parsing (with custom parsing patterns given by user or operator) and 
forwarding to one or multiple syslog endpoint in rfc 5424 syslog format.
Take care that logs-service-broker will always provide json encoded format to final syslogs endpoint(s).

It is for now tied to cloud foundry for different types of logs received by this platform.

This is compliant with the spec [open service broker api](https://www.openservicebrokerapi.org/) for 
[syslog drain](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md#log-drain).

## How to deploy

On cloud foundry service should **not** be deployed from this source code but it must use the boshrelease related to it 
which can be found here: https://github.com/orange-cloudfoundry/logservice-boshrelease/

1. Clone the repo
2. Go build the repo directly this will give you a logs-service-broker runnable server
3. Create a `config.yml` file to set a your configuration. Configuration is explained in [configuration](#configuration) section

## Configuration

For understanding config definition format:
- `[]` means optional (by default parameter is required)
- `<>` means type to use

### Root configuration in config.yml

```yaml
# Port for listening in http
[ port: <int> | default = 8088 ]
# Port for listening in https
# This will be in addition of listening in http
# You will need to set ssl_cert_file and ssl_key_file parameters in addition
[ tls_port: <int> ]
# If set to true, forged url for syslog drain endpoint in broker will always use https url
# This will not let user be able to chose to forward in http or https
[ prefer_tls: <boolean> ]
# Path to the cert file to use for listening in https
[ ssl_cert_file: <string> ]
# Path to the private key file to use for listening in https
[ ssl_key_file: <string> ]
# External url which give docs url to user
# This is an url pointing on this service of course
# using it let you separate logs part from user part
[ external_url: <string> ]
# Set to true to not allow to send logs to external url
# If you want clear separation between user and logs path set it to true
[ disallow_logs_from_external: <boolean> ]
# this is url used to forge url on syslog drain endpoint
# this is an url pointing on this service of course
# e.g.: my-logservice.com
# note: you don't need to set scheme as http or https like it is dependant of parameter `prefer_tls` or user directive
syslog_drain_url: <string>
# If set to true forged url will take the format <service-uuid>.<syslog_drain_url> instead of path style <syslog_drain_url>/<service-uuid>
[ virtual_host: <boolean> ]
# By default user can chose what kind of data to retrieve between `logs`, `metrics` or `all` when creating service
# This will disallow what kind of data user can ask
[ disable_drain_type: <boolean> ]
# Broker username basic auth
broker_username: <string>
# Broker password basic auth
broker_password: <string>
# log level to use for server
# you can chose: `trace`, `debug`, `info`, `warn`, `error`, `fatal` or `panic`
[ log_level: <string> | default = "info" ]
# Set to true to see logs server as json format
[ log_json: <boolean> ]
# Set to true to force not have color when seeing logs
[ log_no_color: <boolean> ]
# Set to true to fallback to sqlite if no database connection is given
# In this case sqlite_path must be set
[ fallback_to_sqlite: <boolean> ]
# if you want to use sqlite with `fallback_to_sqlite` you must set a filepath to store your database
# e.g.: logservice.db
[ sqlite_path: <string> ]
# Set the maximum number of connections in the idle
[ sql_cnx_max_idle: <int> ]
# Set the maximum number of open connections to the database
[ sql_cnx_max_open: <int> ]
# Set the maximum amount of time a connection may be reused
# this is a duration which must be write like: `1h` for one hour or `1d` for one day ...
sql_cnx_max_life: <string>
# By default ping are performed on the database
# if ping failed this will automatically stop the service 
# In a cloud foundry with bosh context this the best behaviour to have to let monit manage the process
# if you're not using boshrelease you should put this parameter to true
[ not_exit_when_con_failed: <boolean> ]
# In order to avoid a lot of request to the database when forwarding logs a cache system is provided
# We recommend to set this value and set it to `always`
# This can be a duration which must be write like: `1h` for one hour or `1d` for one day ... 
# Or this can be set to `always` to keep in cache all requested data indefinitely until data still exists 
# in db to avoid too much memory usage (check is performed each 24h if set to always)
[ cache_duration: <string> ]
# see below for configuration of this part
parsing_keys: 
[ - <parsing_key> ]
# see below for configuration of this part
syslog_addresses:
[ - <syslog_addresse> ]
```

### syslog_addresse configuration

```yaml
# Unique id of your service definition for this parser and forwarder
id: <string>
# Name of your service definition for this parser and forwarder
name: <string>
# Set a different company id to be send in log as sd params.
# This must follow syntax: object@enterprise-number
# Note that 1368 is the orange enterprise number, international enterprise number can be found at: 
# https://www.iana.org/assignments/enterprise-numbers/enterprise-numbers
[ company_id: <string> | default = logsbroker@1368 ]
# Description of your service definition for this parser and forwarder
[ description: <string> ]
# Additional information about your service
# You can describe tags that you want a user set or can set when creating an instance
bullets:
[ - <string> ]
# Urls to forward parsed data in rfc 5424 format with content encoded in json
# at least one url is mandatory. If you set more data will be send to all endpoints in parallel
# this can be:
# - https, e.g.: https://my.http.server.com/receive (data will given in POST param)
# - tcp, e.g.: tcp://my.syslog.server.com:514
# - udp, e.g.: udp://my.syslog.server.com:514
# - tcp with tls, e.g.: tcp+tls://my.syslog.server.com:514. This one accept get parameter for changing behaviour on certificate.
#   you can set `verify=false` to not verify tls cert and `cert=path/to/a/ca/file` to give your own self ca.
#   e.g.: tcp+tls://my.syslog.server.com:514?verify=false
urls:
- <string>
[ - ... ]
# You can set default tags to be send with this parameters which will be formatted as structured data in syslog.
# Tags will be send as follow: <7>1 2016-02-28T09:57:10.80Z org-1.space-1.app-1 someapp [RTR/0] - [logsbroker@1368 tagkey="tag value"]
#                                                                                                            here  ^^^^^^^^^^^^^^^^^^
# You have access to tags templating for dynamics formatting tags, see next section: tags templating
tags:
[ - <string>: <string> ]
# You can set default patterns for parsing. 
# This must be in grok format: https://streamsets.com/documentation/datacollector/latest/help/datacollector/UserGuide/Apx-GrokPatterns/GrokPatterns_title.html
patterns:
[ - <string> ]
# Default drain type (or forced one if `disable_drain_type` param as been set to true)
# There is 3 drain types: `logs`, `metrics` or `all`
# If not set or empty, this is the `logs` drain type which will be used
[ default_drain_type: <string> ]
# Services is only used to define you database
# Behind the hood, logs-service-broker use gautocloud for connecting to a database: https://github.com/cloudfoundry-community/gautocloud
# You can either configure your database as shown in gautocloud but for fastest setup you can set this param
services:
  - name: database
    tags: [database]
    credentials:
      # uri of your database
      # for a mysql/mariadb database set 
      # mysql://root@localhost:3306/logservice
      # postgres or sqlite are also allowed with scheme: postgres:// and sqlite://
      uri: <string>
```

**Note**: Default grok patterns can be found at [/parser/patterns.go](/parser/patterns.go) and [/vendor/github.com/ArthurHlt/grok/patterns.go](/vendor/github.com/ArthurHlt/grok/patterns.go).

#### tags templating

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

In addition you can use those functions for helping you:
- `split <param> <delimiter>`: Split string by a delimiter to get a slice  
- `join <param> <delimiter>`: Make string from a slice collapse by delimiter
- `trimSuffix <param> <suffix>`: Remove suffix from param
- `trimPrefix <param> <prefix>`: Remove prefix from param
- `hasPrefix <param> <prefix>`: Check if prefix exists in param
- `hasSuffix <param> <prefix>`: Check if suffix exists in param
- `ret access.to.value.from.key`: Get the value of a key in a map by exploring it in dot format, e.g: 
this `{"foo": {"exists": ["my-value"]}` can be take by doing `ret "foo.exists.0"`

**tips**: on `ret` function you can use special key `first` and `last` on a slice for respectively the first value of a slice or the last one.

### parsing_key configuration

Some of key/value pair have special effect, those pairs defined will be use as parsing value until there is nothing to parse anymore, we call them parsing keys.

By default parsing keys are:
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

You define parsing key as follow:

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

## Architecture in a cloud foundry context

[![archi](/docs/archi.png)](/docs/archi.png)
