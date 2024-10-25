package main

import (
	"os"
	"path/filepath"
	"reflect"
)

func getStringValue(field reflect.Value) string {
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return ""
		}
		field = field.Elem()
	}
	return field.String()
}

// GetQueryFromFile reads a SQL query from a file and returns it as a string
func GetQueryFromFile(relativePath string) (string, error) {
	queryPath, err := filepath.Abs(relativePath)
	if err != nil {
		return "", err
	}

	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		return "", err
	}
	return string(queryBytes), nil
}
