package utils

import (
	"math"
	"strconv"
)

func Atoi(s string) int {
	r, _ := strconv.Atoi(s)
	return r
}

func ParseFloat(s string) float64 {
	r, _ := strconv.ParseFloat(s, 64)
	return r
}

func MergeMap(parent, partial map[string]interface{}) map[string]interface{} {
	for k, v := range partial {
		if _, ok := parent[k]; !ok {
			parent[k] = v
			continue
		}
		if vMap, ok := v.(map[string]interface{}); ok {
			parent[k] = MergeMap(parent[k].(map[string]interface{}), vMap)
			continue
		}
		parent[k] = v
	}
	return parent
}

func Round(f float64) float64 {
	return math.Floor(f + .5)
}

func RoundPlus(f float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return Round(f*shift) / shift
}
