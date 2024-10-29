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
	rp.log.Debug().
		Str("fieldType", field.Type().String()).
		Msg("Checking date filter")

	if field.Type().String() != "fhir.Date" {
		rp.log.Debug().Msg("Field is not a date")
		return &FilterResult{Passed: false, Message: "field is not a date"}, nil
	}

	dateVal := field.Interface().(fhir.Date)
	filterDate, err := time.Parse("2006-01-02", param.Value)
	if err != nil {
		rp.log.Error().Err(err).Msg("Error parsing filter date")
		return nil, err
	}

	fieldDate := dateVal.Time()
	rp.log.Debug().
		Str("fieldDate", fieldDate.String()).
		Str("filterDate", filterDate.String()).
		Str("comparator", param.Comparator).
		Msg("Comparing dates")

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

	rp.log.Debug().Bool("passed", passed).Msg("Date filter result")
	return &FilterResult{Passed: passed}, nil
}

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
		Str("fieldType", field.Type().String()).
		Str("system", system).
		Str("code", code).
		Msg("Checking token filter")

	// Handle slice types first
	if field.Kind() == reflect.Slice {
		return rp.checkSliceToken(field, system, code)
	}

	// Handle non-slice types
	return rp.checkSingleToken(field, system, code)
}
func (rp *ResourceProcessor) checkSliceToken(slice reflect.Value, system, code string) (*FilterResult, error) {
	// Empty slice never matches
	if slice.Len() == 0 {
		return &FilterResult{Passed: false}, nil
	}

	// Check each element in the slice
	for i := 0; i < slice.Len(); i++ {
		element := slice.Index(i)
		if result, err := rp.checkSingleToken(element, system, code); err != nil {
			return nil, err
		} else if result.Passed {
			return &FilterResult{Passed: true}, nil
		}
	}

	return &FilterResult{Passed: false}, nil
}

func (rp *ResourceProcessor) checkSingleToken(field reflect.Value, system, code string) (*FilterResult, error) {
	// Get the field type, handling pointers
	fieldType := field.Type().String()
	if strings.HasPrefix(fieldType, "*") {
		if field.IsNil() {
			return &FilterResult{Passed: false}, nil
		}
		field = field.Elem()
		fieldType = field.Type().String()
	}

	switch fieldType {
	case "fhir.CodeableConcept":
		codings := field.FieldByName("Coding")
		if !codings.IsValid() {
			return &FilterResult{Passed: false}, nil
		}
		// Recursively check the slice of codings
		return rp.checkSliceToken(codings, system, code)

	case "fhir.Coding":
		return &FilterResult{Passed: rp.matchesCoding(field, system, code)}, nil

	case "fhir.Identifier":
		return rp.checkIdentifierFilter(field, system, code)

	default:
		// Handle simple string comparison
		if field.Kind() == reflect.String {
			return &FilterResult{Passed: field.String() == code}, nil
		}
		return &FilterResult{Passed: false}, nil
	}
}

func (rp *ResourceProcessor) matchesCoding(coding reflect.Value, system, code string) bool {
	var codingSystem, codingCode string

	if systemField := coding.FieldByName("System"); systemField.IsValid() && !systemField.IsNil() {
		codingSystem = systemField.Elem().String()
	}

	if codeField := coding.FieldByName("Code"); codeField.IsValid() && !codeField.IsNil() {
		codingCode = codeField.Elem().String()
	}

	matches := (system == "" || codingSystem == system) && codingCode == code

	// Add logging
	rp.log.Debug().
		Str("codingSystem", codingSystem).
		Str("codingCode", codingCode).
		Str("filterSystem", system).
		Str("filterCode", code).
		Bool("matches", matches).
		Msg("Coding match result")

	return matches
}

// Helper function to check if a single element matches token criteria
func (rp *ResourceProcessor) matchesToken(field reflect.Value, system, code string) bool {
	rp.log.Debug().
		Str("fieldType", field.Type().String()).
		Str("fieldKind", field.Kind().String()).
		Str("system", system).
		Str("code", code).
		Msg("Checking token filter")

	// Handle Coding type
	if field.Type().String() == "fhir.Coding" {
		var coding *fhir.Coding
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				rp.log.Debug().Msg("Field is nil pointer")
				return false
			}
			coding = field.Interface().(*fhir.Coding)
		} else {
			coding = field.Addr().Interface().(*fhir.Coding)
		}

		var codingSystem, codingCode string
		if coding.System != nil {
			codingSystem = *coding.System
		}
		if coding.Code != nil {
			codingCode = *coding.Code
		}

		matches := (system == "" || codingSystem == system) && codingCode == code
		rp.log.Debug().
			Str("codingSystem", codingSystem).
			Str("codingCode", codingCode).
			Bool("matches", matches).
			Msg("Coding type match result")
		return matches
	}

	// Handle string type
	if field.Kind() == reflect.String {
		matches := field.String() == code
		rp.log.Debug().
			Str("fieldValue", field.String()).
			Bool("matches", matches).
			Msg("String type match result")
		return matches
	}

	rp.log.Debug().Msg("Unsupported field type for token filter")
	return false
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
