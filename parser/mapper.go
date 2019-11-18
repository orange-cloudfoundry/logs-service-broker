package parser

import (
	"github.com/orange-cloudfoundry/logs-service-broker/utils"
	"regexp"
)

func Mapper(m map[string]interface{}) map[string]interface{} {
	data := make(map[string]interface{})
	for k, v := range m {
		data = utils.MergeMap(data, Mapkv(k, v))
	}
	return data
}

func Mapkv(k string, v interface{}) map[string]interface{} {
	if vStr, ok := v.(string); ok && vStr != "" &&
		(vStr[0] == '"' && vStr[len(vStr)-1] == '"' || vStr[0] == '\'' && vStr[len(vStr)-1] == '\'') {
		if len(vStr)-2 <= 0 {
			v = ""
		} else {
			v = vStr[1 : len(vStr)-1]
		}
	}
	if k[0] != '[' {
		return map[string]interface{}{
			k: v,
		}
	}
	r := regexp.MustCompile(`\[([^\]]*)\]`)
	sub := r.FindAllStringSubmatch(k, -1)

	d := make(map[string]interface{})
	parent := d
	for i, s := range sub {
		if len(sub)-1 == i {
			d[s[1]] = v
			break
		}
		d[s[1]] = make(map[string]interface{})
		d = d[s[1]].(map[string]interface{})
	}
	return parent
}
