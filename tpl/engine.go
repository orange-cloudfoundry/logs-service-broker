package tpl

import (
	"bytes"
	"text/template"

	"github.com/bluele/gcache"
)

var cachedTemplates = gcache.New(70).LFU().Build()

func loadOrStoreTemplate(v string) (*template.Template, error) {
	tplRaw, err := cachedTemplates.Get(v)
	if err == nil {
		return tplRaw.(*template.Template), nil
	}
	tpl, err := template.New("templater").Funcs(builtins).Parse(v)
	if err != nil {
		return nil, err
	}
	err = cachedTemplates.Set(v, tpl)
	return tpl, err
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
		buf := &bytes.Buffer{}
		tpl, err := loadOrStoreTemplate(v)
		if err != nil {
			return result, err
		}
		tpl.Execute(buf, t.data)

		result[k] = buf.String()
	}

	return result, nil
}
