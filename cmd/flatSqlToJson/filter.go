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
		Str("structfieldType", structFieldValue.Type().Name()).
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
	log.Debug().Str("field", fhirPath).Str("fieldType", field.Type().Name()).Msg("Filtering struct field")
	switch field.Type().Name() {
	case "Identifier", "CodeableConcept", "Coding":
		return filterTokenField(field, searchParameter, fhirPath, log)
	case "Time":
		return filterDateOrDateTimeField(field, searchParameter, fhirPath)
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

// parseDateFilter parses a date filter value, handling prefixes
func parseDateFilter(value string) (string, string) {
	prefixes := []string{"eq", "ne", "gt", "lt", "ge", "le"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return prefix, strings.TrimPrefix(value, prefix)
		}
	}
	return "eq", value // default to equality if no prefix
}

func filterDateOrDateTimeField(field reflect.Value, searchParameter SearchParameter, fhirPath string) (*FilterResult, error) {
	prefix, dateStr := parseDateFilter(searchParameter.Value)

	// Parse the search date
	searchDate, err := parsePartialDate(dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid search date format: %v", err)
	}

	// Get the field's date value
	fieldDateOrDateTime, err := getDateOrDateTimeValue(field)
	if err != nil {
		return nil, fmt.Errorf("error getting field date: %v", err)
	}

	// Compare dates based on prefix
	passed := compareDateTimes(fieldDateOrDateTime, searchDate, prefix)

	return &FilterResult{
		Passed:  passed,
		Message: fmt.Sprintf("Date comparison %s: field=%v search=%v", prefix, fieldDateOrDateTime.Format("2006-01-02T15:04:05.000Z"), searchDate.Format("2006-01-02T15:04:05.000Z")),
	}, nil
}

// parsePartialDate handles parsing of partial dates (YYYY, YYYY-MM, YYYY-MM-DD)
func parsePartialDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006",
		"2006-01",
		"2006-01-02",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported date format: %s", dateStr)
}
func setFieldToZeroIfNotEmpty(field reflect.Value) {
	if !field.IsZero() {
		field.Set(reflect.Zero(field.Type()))
	}
}

// getDateOrDateTimeValue extracts the time.Time value from either a Date or DateTime field
func getDateOrDateTimeValue(field reflect.Value) (time.Time, error) {
	if !field.IsValid() {
		return time.Time{}, fmt.Errorf("invalid field")
	}

	fieldType := field.Type().Name()
	switch fieldType {
	case "DateTime":
		return field.FieldByName("Time").Interface().(time.Time), nil
	case "Date":
		return field.FieldByName("Time").Interface().(time.Time), nil
	case "Time":
		// Handle raw time.Time fields
		if t, ok := field.Interface().(time.Time); ok {
			return t, nil
		}
		return time.Time{}, fmt.Errorf("could not convert Time field to time.Time")
	default:
		return time.Time{}, fmt.Errorf("field type must be Date, DateTime, or Time, got: %s", fieldType)
	}
}

// compareDateTimes compares two date/datetimes based on the comparison prefix
func compareDateTimes(fieldDateTime, searchDateTime time.Time, prefix string) bool {
	println(prefix)
	switch prefix {
	case "eq":
		return fieldDateTime.Equal(searchDateTime)
	case "ne":
		return !fieldDateTime.Equal(searchDateTime)
	case "gt":
		return fieldDateTime.After(searchDateTime)
	case "lt":
		return fieldDateTime.Before(searchDateTime)
	case "ge":
		return fieldDateTime.After(searchDateTime) || fieldDateTime.Equal(searchDateTime)
	case "le":
		return fieldDateTime.Before(searchDateTime) || fieldDateTime.Equal(searchDateTime)
	default:
		return false
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
