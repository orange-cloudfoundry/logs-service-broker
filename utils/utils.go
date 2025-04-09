package utils

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

const delimLastElem = "last"
const delimFirstElem = "first"

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

func CopyMapString(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	newMap := make(map[string]string)
	for k, v := range src {
		newMap[k] = v
	}
	return newMap
}

func Round(f float64) float64 {
	return math.Floor(f + .5)
}

func RoundPlus(f float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return Round(f*shift) / shift
}

func foundVarDelimMap(m map[string]interface{}, delim string) interface{} {
	delimSplit := strings.Split(delim, ".")
	v, ok := m[delimSplit[0]]
	if !ok {
		return nil
	}
	if len(delimSplit) == 1 {
		return v
	}
	return FoundVarDelim(v, strings.Join(delimSplit[1:], "."))
}

func foundVarSlice(s []interface{}, delim string) interface{} {
	if len(s) == 0 {
		return nil
	}
	delimSplit := strings.Split(delim, ".")
	var v interface{}
	start := strings.ToLower(delimSplit[0])
	if start == delimLastElem {
		v = s[len(s)-1]
	} else if start == delimFirstElem {
		v = s[0]
	} else {
		i, _ := strconv.Atoi(start)
		v = s[i]
	}
	if len(delimSplit) == 1 {
		return v
	}
	return FoundVarDelim(v, strings.Join(delimSplit[1:], "."))
}

func FoundVarDelim(elem interface{}, delim string) interface{} {
	_, isSlice := elem.([]interface{})
	_, isMap := elem.(map[string]interface{})
	if isSlice {
		return foundVarSlice(elem.([]interface{}), delim)
	}
	if isMap {
		return foundVarDelimMap(elem.(map[string]interface{}), delim)
	}
	return nil
}

func CreateMapFromDelim(delim string, value interface{}) map[string]interface{} {
	delimSplit := strings.Split(delim, ".")
	if len(delimSplit) == 1 {
		return map[string]interface{}{
			delimSplit[0]: value,
		}
	}
	return map[string]interface{}{
		delimSplit[0]: CreateMapFromDelim(strings.Join(delimSplit[1:], "."), value),
	}
}

// CloseAndLogError is an utility function to close an io.Closer and log errors without returning them
func CloseAndLogError(closer io.Closer) {
	if closer == nil {
		return
	}

	// Attempt to close the resource (e.g., an HTTP response or a file).
	// If an error occurs during the close operation, the error is captured.
	err := closer.Close()
	if err != nil {
		fmt.Printf("Error closing resource: %v", err)
	}
}

// RemoveDir is an utility function to simplified function to remove a directory and log any errors
func RemoveDir(path string) {
	err := os.RemoveAll(path)
	if err != nil {
		fmt.Printf("Error removing directory %s: %v", path, err)
	}
}
