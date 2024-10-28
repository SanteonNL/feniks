package main

import (
	"fmt"
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

	rp.log.Debug().
		Str("path", path).
		Str("fieldType", field.Type().String()).
		Str("fieldKind", field.Kind().String()).
		Str("paramType", param.Type).
		Msg("Checking filter")

	// Handle pointer types
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return &FilterResult{Passed: false}, nil
		}
		field = field.Elem()
	}

	switch param.Type {
	case "token":
		return rp.checkTokenFilter(field, param)
	case "date":
		return rp.checkDateFilter(field, param)
	default:
		return &FilterResult{Passed: true}, nil
	}
}

func (rp *ResourceProcessor) checkDateFilter(field reflect.Value, param SearchParameter) (*FilterResult, error) {
	rp.log.Debug().Str("field.Type().String()", field.Type().String()).Msg("Checking date filter")

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

func (rp *ResourceProcessor) checkTokenFilter(field reflect.Value, param SearchParameter) (*FilterResult, error) {
	system, code := parseTokenValue(param.Value)
	rp.log.Debug().
		Str("system", system).
		Str("code", code).
		Str("fieldType", field.Type().String()).
		Msg("Checking token filter")

	// Get the type string, handling both pointer and non-pointer types
	fieldType := field.Type().String()
	if strings.HasPrefix(fieldType, "*") {
		fieldType = fieldType[1:] // Remove the pointer prefix
	}

	switch fieldType {
	case "fhir.Identifier":
		return rp.checkIdentifierFilter(field, system, code)
	// case "fhir.CodeableConcept":
	// // 	return rp.checkCodeableConceptFilter(field, system, code)
	// case "fhir.Coding":
	// 	return rp.checkCodingFilter(field, system, code)
	default:
		if field.Kind() == reflect.String {
			passed := field.String() == code
			return &FilterResult{Passed: passed}, nil
		}
		return &FilterResult{Passed: false, Message: fmt.Sprintf("Unsupported token field type: %s", fieldType)}, nil
	}
}

func (rp *ResourceProcessor) checkIdentifierFilter(field reflect.Value, system, code string) (*FilterResult, error) {
	var identifier *fhir.Identifier

	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return &FilterResult{Passed: false}, nil
		}
		identifier = field.Interface().(*fhir.Identifier)
	} else {
		identifier = field.Addr().Interface().(*fhir.Identifier)
	}

	// Get system and value, handling nil cases
	var identifierSystem, identifierValue string
	if identifier.System != nil {
		identifierSystem = *identifier.System
	}
	if identifier.Value != nil {
		identifierValue = *identifier.Value
	}

	// Match logic:
	// - If system is provided, both system and code must match
	// - If only code is provided, only value needs to match
	matches := (system == "" || identifierSystem == system) &&
		identifierValue == code

	rp.log.Debug().
		Str("fieldSystem", identifierSystem).
		Str("fieldValue", identifierValue).
		Str("filterSystem", system).
		Str("filterValue", code).
		Bool("matches", matches).
		Msg("Comparing identifier")

	return &FilterResult{Passed: matches}, nil
}

// func (rp *ResourceProcessor) checkCodeableConceptFilter(field reflect.Value, system, code string) (*FilterResult, error) {
// 	if field.IsNil() {
// 		return &FilterResult{Passed: false}, nil
// 	}

// 	concept := field.Interface().(*fhir.CodeableConcept)
// 	if concept.Coding == nil {
// 		return &FilterResult{Passed: false}, nil
// 	}

// 	// Check each coding in the CodeableConcept
// 	for _, coding := range concept.Coding {
// 		if coding == (fhir.Coding{}) {
// 			continue
// 		}

// 		var codingSystem, codingCode string
// 		if coding.System != nil {
// 			codingSystem = *coding.System
// 		}
// 		if coding.Code != nil {
// 			codingCode = *coding.Code
// 		}

// 		// Match if either:
// 		// 1. System is empty and code matches
// 		// 2. Both system and code match
// 		if (system == "" && codingCode == code) ||
// 			(codingSystem == system && codingCode == code) {
// 			return &FilterResult{Passed: true}, nil
// 		}
// 	}

// 	return &FilterResult{Passed: false}, nil
// }

func (rp *ResourceProcessor) checkCodingFilter(field reflect.Value, system, code string) (*FilterResult, error) {
	if field.IsNil() {
		return &FilterResult{Passed: false}, nil
	}

	coding := field.Interface().(*fhir.Coding)

	var codingSystem, codingCode string
	if coding.System != nil {
		codingSystem = *coding.System
	}
	if coding.Code != nil {
		codingCode = *coding.Code
	}

	matches := (system == "" || codingSystem == system) &&
		codingCode == code

	return &FilterResult{Passed: matches}, nil
}
