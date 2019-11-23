package parser

import (
	"strings"

	"github.com/orange-cloudfoundry/logs-service-broker/utils"
)

func Mapper(m map[string]interface{}) map[string]interface{} {
	data := make(map[string]interface{})
	for k, v := range m {
		data = utils.MergeMap(data, Mapkv(k, v))
	}
	return data
}

func Mapkv(k string, v interface{}) map[string]interface{} {
	k = strings.TrimSpace(k)
	if vStr, ok := v.(string); ok && vStr != "" &&
		(vStr[0] == '"' && vStr[len(vStr)-1] == '"' || vStr[0] == '\'' && vStr[len(vStr)-1] == '\'') {
		if len(vStr)-2 <= 0 {
			v = ""
		} else {
			v = vStr[1 : len(vStr)-1]
		}
	}

	if len(k) <= 2 || k[0] != '[' || k[len(k)-1] != ']' {
		return map[string]interface{}{
			k: v,
		}
	}

	sub := strings.Split(k[1:len(k)-1], "][")
	d := make(map[string]interface{})
	parent := d
	for i, s := range sub {
		if len(sub)-1 == i {
			d[s] = v
			break
		}
		d[s] = make(map[string]interface{})
		d = d[s].(map[string]interface{})
	}
	return parent
}
