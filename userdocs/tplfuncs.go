package userdocs

import (
	"bytes"
	"fmt"
	"html/template"
	"reflect"
	"strings"

	"github.com/russross/blackfriday/v2"
)

var tplfuncs = template.FuncMap{
	"split":      split,
	"join":       join,
	"slug":       slug,
	"toUpper":    toUpper,
	"toLower":    toLower,
	"trimSuffix": trimSuffix,
	"trimPrefix": trimPrefix,
	"hasPrefix":  hasPrefix,
	"hasSuffix":  hasSuffix,
	"title":      title,
	"safe":       safe,
	"markdown":   markdown,
	"rawContent": rawContent,
	"parse":      parse,
	"len":        lenValue,
}

func split(a, delimiter string) ([]string, error) {
	return strings.Split(a, delimiter), nil
}

func toUpper(a string) string {
	return strings.ToUpper(a)
}

func slug(a string) string {
	a = strings.Replace(strings.ToLower(a), "_", "-", -1)
	a = strings.Replace(a, " ", "-", -1)
	return a
}

func toLower(a string) string {
	return strings.ToLower(a)
}

func join(a []string, separator string) (string, error) {
	return strings.Join(a, separator), nil
}

func trimSuffix(suffix, s string) (string, error) {
	return strings.TrimSuffix(s, suffix), nil
}

func trimPrefix(prefix, s string) (string, error) {
	return strings.TrimPrefix(s, prefix), nil
}

func hasPrefix(s, prefix string) (bool, error) {
	return strings.HasPrefix(s, prefix), nil
}

func hasSuffix(s, suffix string) (bool, error) {
	return strings.HasSuffix(s, suffix), nil
}

func title(s string) string {
	return strings.Title(s)
}

func safe(s interface{}) template.HTML {
	return template.HTML(fmt.Sprint(s))
}

func markdown(s interface{}) template.HTML {
	exts := blackfriday.NoIntraEmphasis | blackfriday.Tables | blackfriday.FencedCode |
		blackfriday.Autolink | blackfriday.Strikethrough | blackfriday.SpaceHeadings | blackfriday.HeadingIDs |
		blackfriday.BackslashLineBreak | blackfriday.DefinitionLists | blackfriday.AutoHeadingIDs | blackfriday.NoEmptyLineBeforeBlock
	return template.HTML(
		blackfriday.Run([]byte(fmt.Sprint(s)),
			blackfriday.WithExtensions(exts),
		))
}

func rawContent(s interface{}) (template.HTML, error) {
	tplTxt, err := boxTemplates.FindString(fmt.Sprint(s))
	return template.HTML(tplTxt), err
}

func parse(s, value interface{}) (template.HTML, error) {
	buf := &bytes.Buffer{}
	err := mainTpl.ExecuteTemplate(buf, fmt.Sprint(s), value)
	if err != nil {
		return template.HTML(""), err
	}
	return template.HTML(buf.String()), nil
}

func lenValue(s interface{}) int {
	return reflect.ValueOf(s).Len()
}
