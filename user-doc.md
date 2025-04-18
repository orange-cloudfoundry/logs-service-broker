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
$ cf update-service logs <plan> my-log-service -c '{"tags": {"my-tag": "bar"}, "patterns": ["%{GREEDYDATA:my-data}"]}'
$ cf unbind-service <my-app> my-log-service
$ cf bind-service <my-app> my-log-service
```

**Note**: Unbinding is necessary for logservice to consider your new changes

## Available parameters

When creating or updating service these parameters can be passed:
- `tags` (*Map key value*): Define your tags (see tags formatting in [tags formatting section](#tags-formatting))
- `patterns` (*Slice of string*): Define your patter (see patterns and grok available patterns in [patterns formatting section](#patterns-formatting))
- `drain_type` (*can be `logs` (similar to empty), `metrics` or `all`*, usable if operator didn't disallow metrics with `disable_drain_type` in config ): Allow metrics or both logs and metrics to be sent in logservice.
(**Warning** Metrics should be use when you have not prometheus, a lot of dashboards are already available on it)
- `use_tls` (*boolean*, usable if operator not set `prefer_tls` in config ): Set to `true` for making cloud foundry send logs encrypted to logservice


## Tags formatting

Tags can be dynamically be formatted by using golang templating:
```json
{
  "tags": {
    "my-tag": "{{ .App }}-my-tag"
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
this `{"foo": {"exists": ["my-value"]}` can be taken by doing `ret "foo.exists.0"`

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
- <other parsing keys given by operator>...

It is always good to set one of this key in your pattern.

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

## Patterns keys

- **USERNAME**: `[a-zA-Z0-9._-]+`
- **USER**: `%{USERNAME}`
- **EMAILLOCALPART**: `[a-zA-Z][a-zA-Z0-9_.+-=:]+`
- **EMAILADDRESS**: `%{EMAILLOCALPART}@%{HOSTNAME}`
- **HTTPDUSER**: `%{EMAILADDRESS}|%{USER}`
- **INT**: `(?:[+-]?(?:[0-9]+))`
- **BASE10NUM**: `([+-]?(?:[0-9]+(?:\.[0-9]+)?)|\.[0-9]+)`
- **NUMBER**: `(?:%{BASE10NUM})`
- **BASE16NUM**: `(0[xX]?[0-9a-fA-F]+)`
- **POSINT**: `\b(?:[1-9][0-9]*)\b`
- **NONNEGINT**: `\b(?:[0-9]+)\b`
- **WORD**: `\b\w+\b`
- **NOTSPACE**: `\S+`
- **SPACE**: `\s*`
- **DATA**: `.*?`
- **GREEDYDATA**: `.*`
- **QUOTEDSTRING**: `"([^"\\]*(\\.[^"\\]*)*)"|\'([^\'\\]*(\\.[^\'\\]*)*)\'`
- **UUID**: `[A-Fa-f0-9]{8}-(?:[A-Fa-f0-9]{4}-){3}[A-Fa-f0-9]{12}`
- **MAC**: `(?:%{CISCOMAC}|%{WINDOWSMAC}|%{COMMONMAC})`
- **CISCOMAC**: `(?:(?:[A-Fa-f0-9]{4}\.){2}[A-Fa-f0-9]{4})`
- **WINDOWSMAC**: `(?:(?:[A-Fa-f0-9]{2}-){5}[A-Fa-f0-9]{2})`
- **COMMONMAC**: `(?:(?:[A-Fa-f0-9]{2}:){5}[A-Fa-f0-9]{2})`
- **IPV6**: `((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:)))(%.+)?`
- **IPV4**: `(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`
- **IP**: `(?:%{IPV6}|%{IPV4})`
- **HOSTNAME**: `\b(?:[0-9A-Za-z][0-9A-Za-z-]{0,62})(?:\.(?:[0-9A-Za-z][0-9A-Za-z-]{0,62}))*(\.?|\b)`
- **HOST**: `%{HOSTNAME}`
- **IPORHOST**: `(?:%{IP}|%{HOSTNAME})`
- **HOSTPORT**: `%{IPORHOST}:%{POSINT}`
- **PATH**: `(?:%{UNIXPATH}|%{WINPATH})`
- **UNIXPATH**: `(/[\w_%!$@:.,-]?/?)(\S+)?`
- **TTY**: `(?:/dev/(pts|tty([pq])?)(\w+)?/?(?:[0-9]+))`
- **WINPATH**: `([A-Za-z]:|\\)(?:\\[^\\?*]*)+`
- **URIPROTO**: `[A-Za-z]+(\+[A-Za-z+]+)?`
- **URIHOST**: `%{IPORHOST}(?::%{POSINT:port})?`
- **URIPATH**: `(?:/[A-Za-z0-9$.+!*'(){},~:;=@#%_\-]*)+`
- **URIPARAM**: `\?[A-Za-z0-9$.+!*'|(){},~@#%&/=:;_?\-\[\]<>]*`
- **URIPATHPARAM**: `%{URIPATH}(?:%{URIPARAM})?`
- **URI**: `%{URIPROTO}://(?:%{USER}(?::[^@]*)?@)?(?:%{URIHOST})?(?:%{URIPATHPARAM})?`
- **MONTH**: `\b(?:Jan(?:uary|uar)?|Feb(?:ruary|ruar)?|M(?:a|ä)?r(?:ch|z)?|Apr(?:il)?|Ma(?:y|i)?|Jun(?:e|i)?|Jul(?:y)?|Aug(?:ust)?|Sep(?:tember)?|O(?:c|k)?t(?:ober)?|Nov(?:ember)?|De(?:c|z)(?:ember)?)\b`
- **MONTHNUM**: `(?:0?[1-9]|1[0-2])`
- **MONTHNUM2**: `(?:0[1-9]|1[0-2])`
- **MONTHDAY**: `(?:(?:0[1-9])|(?:[12][0-9])|(?:3[01])|[1-9])`
- **DAY**: `(?:Mon(?:day)?|Tue(?:sday)?|Wed(?:nesday)?|Thu(?:rsday)?|Fri(?:day)?|Sat(?:urday)?|Sun(?:day)?)`
- **YEAR**: `(\d\d){1,2}`
- **HOUR**: `(?:2[0123]|[01]?[0-9])`
- **MINUTE**: `(?:[0-5][0-9])`
- **SECOND**: `(?:(?:[0-5]?[0-9]|60)(?:[:.,][0-9]+)?)`
- **TIME**: `([^0-9]?)%{HOUR}:%{MINUTE}(?::%{SECOND})([^0-9]?)`
- **DATE_US**: `%{MONTHNUM}[/-]%{MONTHDAY}[/-]%{YEAR}`
- **DATE_EU**: `%{MONTHDAY}[./-]%{MONTHNUM}[./-]%{YEAR}`
- **ISO8601_TIMEZONE**: `(?:Z|[+-]%{HOUR}(?::?%{MINUTE}))`
- **ISO8601_SECOND**: `(?:%{SECOND}|60)`
- **TIMESTAMP_ISO8601**: `%{YEAR}-%{MONTHNUM}-%{MONTHDAY}[T ]%{HOUR}:?%{MINUTE}(?::?%{SECOND})?%{ISO8601_TIMEZONE}?`
- **DATE**: `%{DATE_US}|%{DATE_EU}`
- **DATESTAMP**: `%{DATE}[- ]%{TIME}`
- **TZ**: `(?:[PMCE][SD]T|UTC)`
- **DATESTAMP_RFC822**: `%{DAY} %{MONTH} %{MONTHDAY} %{YEAR} %{TIME} %{TZ}`
- **DATESTAMP_RFC2822**: `%{DAY}, %{MONTHDAY} %{MONTH} %{YEAR} %{TIME} %{ISO8601_TIMEZONE}`
- **DATESTAMP_OTHER**: `%{DAY} %{MONTH} %{MONTHDAY} %{TIME} %{TZ} %{YEAR}`
- **DATESTAMP_EVENTLOG**: `%{YEAR}%{MONTHNUM2}%{MONTHDAY}%{HOUR}%{MINUTE}%{SECOND}`
- **HTTPDERROR_DATE**: `%{DAY} %{MONTH} %{MONTHDAY} %{TIME} %{YEAR}`
- **SYSLOGTIMESTAMP**: `%{MONTH} +%{MONTHDAY} %{TIME}`
- **PROG**: `[\x21-\x5a\x5c\x5e-\x7e]+`
- **SYSLOGPROG**: `%{PROG:program}(?:\[%{POSINT:pid}\])?`
- **SYSLOGHOST**: `%{IPORHOST}`
- **SYSLOGFACILITY**: `<%{NONNEGINT:facility}.%{NONNEGINT:priority}>`
- **HTTPDATE**: `%{MONTHDAY}/%{MONTH}/%{YEAR}:%{TIME} %{INT}`
- **QS**: `%{QUOTEDSTRING}`
- **SYSLOGBASE**: `%{SYSLOGTIMESTAMP:timestamp} (?:%{SYSLOGFACILITY} )?%{SYSLOGHOST:logsource} %{SYSLOGPROG}:`
- **COMMONAPACHELOG**: `%{IPORHOST:clientip} %{HTTPDUSER:ident} %{USER:auth} \[%{HTTPDATE:timestamp}\] "(?:%{WORD:verb} %{NOTSPACE:request}(?: HTTP/%{NUMBER:httpversion})?|%{DATA:rawrequest})" %{NUMBER:response} (?:%{NUMBER:bytes}|-)`
- **COMBINEDAPACHELOG**: `%{COMMONAPACHELOG} %{QS:referrer} %{QS:agent}`
- **HTTPD20_ERRORLOG**: `\[%{HTTPDERROR_DATE:timestamp}\] \[%{LOGLEVEL:loglevel}\] (?:\[client %{IPORHOST:clientip}\] ){0,1}%{GREEDYDATA:errormsg}`
- **HTTPD24_ERRORLOG**: `\[%{HTTPDERROR_DATE:timestamp}\] \[%{WORD:module}:%{LOGLEVEL:loglevel}\] \[pid %{POSINT:pid}:tid %{NUMBER:tid}\]( \(%{POSINT:proxy_errorcode}\)%{DATA:proxy_errormessage}:)?( \[client %{IPORHOST:client}:%{POSINT:clientport}\])? %{DATA:errorcode}: %{GREEDYDATA:message}`
- **HTTPD_ERRORLOG**: `%{HTTPD20_ERRORLOG}|%{HTTPD24_ERRORLOG}`
- **LOGLEVEL**: `([Aa]lert|ALERT|[Tt]race|TRACE|[Dd]ebug|DEBUG|[Nn]otice|NOTICE|[Ii]nfo|INFO|[Ww]arn?(?:ing)?|WARN?(?:ING)?|[Ee]rr?(?:or)?|ERR?(?:OR)?|[Cc]rit?(?:ical)?|CRIT?(?:ICAL)?|[Ff]atal|FATAL|[Ss]evere|SEVERE|EMERG(?:ENCY)?|[Ee]merg(?:ency)?)`
- **GREEDYQUOTE**: `[^"]*`
- **MODSECCLIENT**: `\[client %{IPORHOST:[modsecurity-error][sourcehost]}\]`
- **MODSECPREFIX**: `ModSecurity: %{NOTSPACE:[modsecurity-error][severity]}\. %{GREEDYDATA:[modsecurity-error][message]}`
- **MODSECRULEFILE**: `\[file %{QUOTEDSTRING:[modsecurity-error][rulefile]}\]`
- **MODSECRULELINE**: `\[line %{QUOTEDSTRING:[modsecurity-error][ruleline]}\]`
- **MODSECMATCHOFFSET**: `\[offset %{QUOTEDSTRING:[modsecurity-error][matchoffset]}\]`
- **MODSECRULEID**: `\[id \"%{NUMBER:[modsecurity-error][ruleid]:int}\"\]`
- **MODSECRULEREV**: `\[rev %{QUOTEDSTRING:[modsecurity-error][rulerev]}\]`
- **MODSECSCOREERROR**: `\"Inbound Anomaly Score Exceeded \(Total Inbound Score: %{NUMBER:[modsecurity-error][score]:int}%{GREEDYQUOTE:[modsecurity-error][rulemessage]}\"`
- **MODSECSCOREAUDIT**: `\[msg \"Inbound Anomaly Score Exceeded \(Total Inbound Score: %{NUMBER:[app][audit_data][score]:int}%{GREEDYQUOTE:[app][audit_data][rulemessage]}\"\]`
- **MODSECRULEMSG**: `%{MODSECSCOREAUDIT}`
- **MODSECRULEMSG2**: `\[msg (?:%{MODSECSCOREERROR}|%{QUOTEDSTRING:[modsecurity-error][rulemessage]})\]`
- **MODSECRULEDATA**: `\[data %{QUOTEDSTRING:[modsecurity-error][ruledata]}\]`
- **MODSECRULESEVERITY**: `\[severity %{QUOTEDSTRING:[modsecurity-error][ruleseverity]}\]`
- **MODSECRULEVERSION**: `\[ver %{QUOTEDSTRING:[modsecurity-error][ruleversion]}\]`
- **MODSECRULEMATURITY**: `\[maturity %{QUOTEDSTRING:[modsecurity-error][rulematurity]}\]`
- **MODSECRULEACCURACY**: `\[accuracy %{QUOTEDSTRING:[modsecurity-error][ruleaccuracy]}\]`
- **MODSECRULETAGS**: `(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag0]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag1]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag2]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag3]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag4]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag5]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag6]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag7]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag8]}\] )?(?:\[tag %{QUOTEDSTRING:[modsecurity-error][ruletag9]}\] )?(?:\[tag %{QUOTEDSTRING}\] )*`
- **MODSECHOSTNAME**: `\[hostname %{QUOTEDSTRING:[modsecurity-error][targethost]}\]`
- **MODSECURI**: `\[uri %{QUOTEDSTRING:[modsecurity-error][targeturi]}\]`
- **MODSECUID**: `\[unique_id %{QUOTEDSTRING:[modsecurity-error][uniqueid]}\]`
- **MODSECAPACHEERROR**: `%{MODSECCLIENT} %{MODSECPREFIX} %{MODSECRULEFILE} %{MODSECRULELINE} (?:%{MODSECMATCHOFFSET} )?(?:%{MODSECRULEID} )?(?:%{MODSECRULEREV} )?(?:%{MODSECRULEMSG2} )?(?:%{MODSECRULEDATA} )?(?:%{MODSECRULESEVERITY} )?(?:%{MODSECRULEVERSION} )?(?:%{MODSECRULEMATURITY} )?(?:%{MODSECRULEACCURACY} )?%{MODSECRULETAGS}%{MODSECHOSTNAME} %{MODSECURI} %{MODSECUID}`
- **RTR**: `%{HOSTNAME:hostname} - \[%{TIMESTAMP_ISO8601:timestamp}\] "%{WORD:verb} %{URIPATHPARAM:path} %{PROG:http_spec}" %{BASE10NUM:status:int} %{BASE10NUM:request_bytes_received:int} %{BASE10NUM:body_bytes_sent:int} "%{GREEDYQUOTE:referer}" "%{GREEDYQUOTE:http_user_agent}" "%{IPORHOST:src_host}:%{POSINT:src_port:int}" "%{IPORHOST:dst_host}:%{POSINT:dst_port:int}" x_forwarded_for:"%{GREEDYQUOTE:x_forwarded_for}" x_forwarded_proto:"%{GREEDYQUOTE:x_forwarded_proto}" vcap_request_id:"%{NOTSPACE:vcap_request_id}" response_time:%{NUMBER:response_time_sec:float} app_id:"%{NOTSPACE:app_id}" app_index:"%{BASE10NUM:app_index:int}"`
- **DATESTAMP_ALT**: `%{DAY} %{MONTH} %{MONTHDAY} %{TIME} %{YEAR}`
- **DATESTAMP_TXT**: `%{MONTHDAY}-%{MONTH}-%{YEAR} %{TIME}`

## App default parsing

- `%{TIME} \|\-%{LOGLEVEL:@level} in %{NOTSPACE:[app][logger]} - %{GREEDYDATA:@message}`
- `\[CONTAINER\]%{SPACE}%{NOTSPACE}%{SPACE}%{LOGLEVEL:@level}%{SPACE}%{GREEDYDATA:@message}`
- `%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}%{HOSTNAME:[app][hostname]} - - \[%{HTTPDATE:[app][timestamp]}\] "%{WORD:[app][verb]} %{URIPATHPARAM:[app][path]} %{PROG:[app][http_spec]}" %{BASE10NUM:[app][status]:int} %{BASE10NUM:[app][request_bytes_received]:int} vcap_request_id=%{NOTSPACE:@request_id} %{GREEDYDATA:@message}`
- `%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}\[%{DATESTAMP_ALT:[app][timestamp]}\] \[(core|mpm_event):%{WORD:@level}\] %{GREEDYDATA:@message}`
- `%{TIME} %{NOTSPACE:[app][program]}%{SPACE}\|%{SPACE}\[%{DATESTAMP_TXT:[app][