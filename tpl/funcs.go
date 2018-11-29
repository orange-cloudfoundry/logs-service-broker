package tpl

import (
	"github.com/spf13/cast"
	"strings"
	"text/template"
)

var builtins = template.FuncMap{
	"split":      split,
	"join":       join,
	"trimSuffix": trimSuffix,
	"trimPrefix": trimPrefix,
	"hasPrefix":  hasPrefix,
	"hasSuffix":  hasSuffix,
}

func split(a interface{}, delimiter string) ([]string, error) {
	aStr, err := cast.ToStringE(a)
	if err != nil {
		return []string{}, err
	}

	return strings.Split(aStr, delimiter), nil
}

func join(a interface{}, separator string) (string, error) {
	aStr, err := cast.ToStringSliceE(a)
	if err != nil {
		return "", err
	}

	return strings.Join(aStr, separator), nil
}

func trimSuffix(suffix, s interface{}) (string, error) {
	ss, err := cast.ToStringE(s)
	if err != nil {
		return "", err
	}

	sx, err := cast.ToStringE(suffix)
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(ss, sx), nil
}

func trimPrefix(prefix, s interface{}) (string, error) {
	ss, err := cast.ToStringE(s)
	if err != nil {
		return "", err
	}

	sx, err := cast.ToStringE(prefix)
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(ss, sx), nil
}

func hasPrefix(s, prefix interface{}) (bool, error) {
	ss, err := cast.ToStringE(s)
	if err != nil {
		return false, err
	}

	sx, err := cast.ToStringE(prefix)
	if err != nil {
		return false, err
	}

	return strings.HasPrefix(ss, sx), nil
}

func hasSuffix(s, suffix interface{}) (bool, error) {
	ss, err := cast.ToStringE(s)
	if err != nil {
		return false, err
	}

	sx, err := cast.ToStringE(suffix)
	if err != nil {
		return false, err
	}

	return strings.HasSuffix(ss, sx), nil
}
