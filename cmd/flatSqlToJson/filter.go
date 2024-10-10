package main

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type SearchParameter struct {
	Code       string   `json:"code"`
	Modifier   []string `json:"modifier,omitempty"`
	Comparator string   `json:"comparator,omitempty"`
	Value      string   `json:"value"`
	Type       string   `json:"type,omitempty"`
	Expression string
}

// Key = Patient.identifier
type SearchParameterMap map[string]SearchParameter

type FilterResult struct {
	Passed  bool
	Message string
}

func ApplyFilter(structPath string, structFieldValue reflect.Value, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Str("structPath", structPath).Msg("Applying filter")

	if len(searchParameterMap) == 0 {
		log.Debug().Msg("SearchParameterMap is empty, passing filter by default")
		return &FilterResult{Passed: true}, nil
	}

	searchParameter, ok := searchParameterMap[structPath]
	if !ok {
		log.Debug().
			Str("Structpath", structPath).
			Msg("No filter found for structPath")
		return &FilterResult{Passed: true, Message: fmt.Sprintf("No filter defined for: %s", structPath)}, nil
	}

	if structFieldValue.Kind() == reflect.Slice {
		// For slices, we delegate to populateSlice which now handles the filtering
		return &FilterResult{Passed: true}, nil
	}

	filterResult, err := determineFilterType(structFieldValue, searchParameter, structPath, log)
	if err != nil {
		return nil, err
	}
	log.Debug().
		Str("structpath", structPath).
		Str("structfield", structFieldValue.Type().Name()).
		Bool("passed", filterResult.Passed).
		Msg("Apply filter result")
	if !filterResult.Passed {
		return &FilterResult{Passed: false, Message: fmt.Sprintf("Field filtered out: %s", structPath)}, nil
	}

	return &FilterResult{Passed: true}, nil
}

func determineFilterType(field reflect.Value, searchParameter SearchParameter, fhirPath string, log zerolog.Logger) (*FilterResult, error) {

	if field.IsZero() {
		log.Debug().Str("field", fhirPath).Msg("Field is already zero value, skipping filtering")
		return &FilterResult{Passed: true}, nil
	}

	switch field.Kind() {
	case reflect.Slice:
		return filterSlice(field, searchParameter, fhirPath, log)
	case reflect.Struct:
		return filterStruct(field, searchParameter, fhirPath, log)
	case reflect.Ptr:
		if field.IsNil() {
			return &FilterResult{Passed: true}, nil
		}
		return determineFilterType(field.Elem(), searchParameter, fhirPath, log)
	default:
		return filterBasicType(field, searchParameter, fhirPath, log)
	}
}

func filterSlice(field reflect.Value, searchParameter SearchParameter, fhirPath string, log zerolog.Logger) (*FilterResult, error) {
	for i := 0; i < field.Len(); i++ {
		result, err := determineFilterType(field.Index(i), searchParameter, fhirPath, log)
		if err != nil {
			return nil, err
		}
		if result.Passed {
			return result, nil // If any element passes, the whole slice passes
		}
	}
	return &FilterResult{Passed: false, Message: fmt.Sprintf("No elements in slice passed filter: %s", fhirPath)}, nil
}

func filterStruct(field reflect.Value, searchParameter SearchParameter, fhirPath string, log zerolog.Logger) (*FilterResult, error) {
	switch field.Type().Name() {
	case "Identifier", "CodeableConcept", "Coding":
		return filterTokenField(field, searchParameter, fhirPath, log)
	default:
		// For other structs, we might want to check nested fields
		for i := 0; i < field.NumField(); i++ {
			result, err := determineFilterType(field.Field(i), searchParameter, fmt.Sprintf("%s.%s", fhirPath, field.Type().Field(i).Name), log)
			if err != nil {
				return nil, err
			}
			if result.Passed {
				return result, nil
			}
		}
		return &FilterResult{Passed: false, Message: fmt.Sprintf("No fields in struct passed filter: %s", fhirPath)}, nil
	}
}

func filterTokenField(field reflect.Value, searchParameter SearchParameter, fhirPath string, log zerolog.Logger) (*FilterResult, error) {
	system, code := parseFilter(searchParameter.Value)
	log.Debug().Str("field", fhirPath).Str("system", system).Str("code", code).Msg("Filtering token field")

	switch field.Type().Name() {
	case "Identifier":
		return matchesIdentifierFilter(field, system, code, fhirPath, log)
	//case "CodeableConcept":
	// 	return matchesCodeableConceptFilter(field, system, code, fhirPath)
	// case "Coding":
	// 	return matchesCodingFilter(field, system, code, fhirPath)
	default:
		return &FilterResult{Passed: false, Message: fmt.Sprintf("Unsupported token field type: %s for field %s", field.Type().Name(), fhirPath)}, nil
	}
}

func matchesIdentifierFilter(v reflect.Value, system, code, fhirPath string, log zerolog.Logger) (*FilterResult, error) {
	systemField := v.FieldByName("System")
	valueField := v.FieldByName("Value")

	if !systemField.IsValid() || !valueField.IsValid() {
		return &FilterResult{Passed: false, Message: fmt.Sprintf("Invalid Identifier structure for field %s", fhirPath)}, nil
	}

	systemValue := getStringValue(systemField)
	valueValue := getStringValue(valueField)

	matches := (system == "" || systemValue == system) && valueValue == code
	log.Debug().
		Str("field", fhirPath).
		Str("fieldSystem", systemValue).
		Str("fieldValue", valueValue).
		Str("filterSystem", system).
		Str("filterValue", code).
		Bool("matches", matches).
		Msg("Comparing identifier")

	if matches {
		return &FilterResult{Passed: true}, nil
	}
	return &FilterResult{Passed: false, Message: fmt.Sprintf("Identifier did not match for field %s", fhirPath)}, nil
}

func filterBasicType(field reflect.Value, searchParameter SearchParameter, fhirPath string, log zerolog.Logger) (*FilterResult, error) {
	// Implement basic type filtering (e.g., string, int, etc.) if needed
	// For now, we'll just pass all basic types
	return &FilterResult{Passed: true}, nil
}

func filterDateField(field reflect.Value, searchParameter SearchParameter, fhirPath string) (bool, error) {
	filterDate, err := time.Parse("2006-01-02", searchParameter.Value)
	if err != nil {
		return false, fmt.Errorf("invalid date format for field %s: %v", fhirPath, err)
	}

	fieldTime, err := getTimeFromField(field)
	if err != nil {
		return false, fmt.Errorf("error getting time from field %s: %v", fhirPath, err)
	}

	switch searchParameter.Comparator {
	case "eq", "":
		return fieldTime.Equal(filterDate), nil
	case "gt":
		return fieldTime.After(filterDate), nil
	case "lt":
		return fieldTime.Before(filterDate), nil
	case "ge":
		return !fieldTime.Before(filterDate), nil
	case "le":
		return !fieldTime.After(filterDate), nil
	default:
		return false, fmt.Errorf("unsupported date comparator for field %s: %s", fhirPath, searchParameter.Comparator)
	}
}

func setFieldToZeroIfNotEmpty(field reflect.Value) {
	if !field.IsZero() {
		field.Set(reflect.Zero(field.Type()))
	}
}

func getTimeFromField(field reflect.Value) (time.Time, error) {

	// Parse the FHIR date string format
	dateString := *field.Interface().(*string)
	return time.Parse(time.RFC3339, dateString)

}

func parseFilter(filter string) (string, string) {
	parts := strings.Split(filter, "|")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}
