package main

import (
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

func ptr(s string) *string {
	return &s
}
