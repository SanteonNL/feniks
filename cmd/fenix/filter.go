package main

import (
	"reflect"
	"strings"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
)

// Part 3: Filter System
func (rp *ResourceProcessor) checkFilter(path string, field reflect.Value) (*FilterResult, error) {
	param, exists := rp.searchParams[path]
	if !exists {
		return &FilterResult{Passed: true}, nil
	}

	switch param.Type {
	case "date":
		return rp.checkDateFilter(field, param)
	// case "token":
	// 	return rp.checkTokenFilter(field, param)
	// case "string":
	// 	return rp.checkStringFilter(field, param)
	// case "number":
	// 	return rp.checkNumberFilter(field, param)
	// case "boolean":
	// 	return rp.checkBooleanFilter(field, param)
	default:
		return &FilterResult{Passed: true}, nil
	}
}

func (rp *ResourceProcessor) checkDateFilter(field reflect.Value, param SearchParameter) (*FilterResult, error) {
	if field.Type().String() != "*fhir.Date" {
		return &FilterResult{Passed: false, Message: "field is not a date"}, nil
	}

	dateVal := field.Interface().(*fhir.Date)
	if dateVal == nil {
		return &FilterResult{Passed: false}, nil
	}

	filterDate, err := time.Parse("2006-01-02", param.Value)
	if err != nil {
		return nil, err
	}

	fieldDate := dateVal.Time()
	passed := false

	switch param.Comparator {
	case "eq", "":
		passed = fieldDate.Equal(filterDate)
	case "gt":
		passed = fieldDate.After(filterDate)
	case "lt":
		passed = fieldDate.Before(filterDate)
	case "ge":
		passed = !fieldDate.Before(filterDate)
	case "le":
		passed = !fieldDate.After(filterDate)
	}

	return &FilterResult{Passed: passed}, nil
}

// func (rp *ResourceProcessor) checkTokenFilter(field reflect.Value, param SearchParameter) (*FilterResult, error) {
// 	system, code := parseTokenValue(param.Value)

// 	// Handle Coding and CodeableConcept types
// 	switch field.Type().Name() {
// 	case "Coding":
// 		return rp.checkCodingFilter(field, system, code)
// 	case "CodeableConcept":
// 		return rp.checkCodeableConceptFilter(field, system, code)
// 	default:
// 		// For simple string comparison
// 		if field.Kind() == reflect.String {
// 			passed := field.String() == code
// 			return &FilterResult{Passed: passed}, nil
// 		}
// 	}

// 	return &FilterResult{Passed: false}, nil
// }

func (rp *ResourceProcessor) checkStringFilter(field reflect.Value, param SearchParameter) (*FilterResult, error) {
	if field.Kind() != reflect.String &&
		(field.Kind() != reflect.Ptr || field.Elem().Kind() != reflect.String) {
		return &FilterResult{Passed: false}, nil
	}

	var fieldValue string
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return &FilterResult{Passed: false}, nil
		}
		fieldValue = field.Elem().String()
	} else {
		fieldValue = field.String()
	}

	passed := fieldValue == param.Value
	return &FilterResult{Passed: passed}, nil
}

func parseTokenValue(value string) (string, string) {
	parts := strings.Split(value, "|")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}
