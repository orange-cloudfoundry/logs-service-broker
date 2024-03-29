package tpl

import (
	"bytes"
	"hash/fnv"
	"strings"
	"sync"
	"text/template"
)

var cachedTemplates = &sync.Map{}

func loadOrStoreTemplate(v string) (*template.Template, error) {
	key, err := hashKey(v)
	if err != nil {
		return nil, err
	}
	tplRaw, hasReceive := cachedTemplates.Load(key)
	if hasReceive && tplRaw != nil {
		return tplRaw.(*template.Template), nil
	}
	tpl, err := template.New("templater").Funcs(builtins).Parse(v)
	if err != nil {
		return nil, err
	}
	if hasReceive {
		return tpl, nil
	}
	cachedTemplates.Store(key, tpl)
	return tpl, nil
}

func hashKey(v string) (uint32, error) {
	h := fnv.New32a()
	_, err := h.Write([]byte(v))
	if err != nil {
		return 0, err
	}
	return h.Sum32(), err
}

type Templater struct {
	data interface{}
}

func NewTemplater(i interface{}) *Templater {
	return &Templater{
		data: i,
	}
}

func (t Templater) Execute(entries map[string]string) (map[string]string, error) {
	if len(entries) == 0 {
		return entries, nil
	}
	result := make(map[string]string)
	for k, v := range entries {
		// do not do templating if there is no inside
		if !strings.Contains(v, "{{") {
			result[k] = v
			continue
		}
		buf := &bytes.Buffer{}
		tpl, err := loadOrStoreTemplate(v)
		if err != nil {
			return result, err
		}
		err = tpl.Execute(buf, t.data)
		if err != nil {
			return result, err
		}
		result[k] = buf.String()
	}

	return result, nil
}
