package processor

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
)

// checkFilter validates a field against search parameters
func (p *ProcessorService) checkFilter(path string, field reflect.Value) (*FilterResult, error) {
	param, exists := p.searchParams[path]
	if !exists {
		return &FilterResult{Passed: true}, nil
	}

	p.log.Debug().
		Str("path", path).
		Str("fieldType", field.Type().String()).
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
		return p.checkTokenFilter(field, param)
	case "date":
		return p.checkDateFilter(field, param)
	default:
		return &FilterResult{Passed: true}, nil
	}
}

// checkTokenFilter handles token-type filters
func (p *ProcessorService) checkTokenFilter(field reflect.Value, param SearchParameter) (*FilterResult, error) {
	// Check if it's a ValueSet reference
	if strings.Contains(param.Value, "ValueSet/") {
		return p.checkValueSetFilter(field, param)
	}

	system, code := parseTokenValue(param.Value)
	return p.checkTokenWithSystemCode(field, system, code)
}

// checkValueSetFilter validates against a ValueSet
func (p *ProcessorService) checkValueSetFilter(field reflect.Value, param SearchParameter) (*FilterResult, error) {
	valueSetURL := strings.TrimPrefix(param.Value, "ValueSet/")

	p.log.Debug().
		Str("valueSetURL", valueSetURL).
		Str("fieldType", field.Type().String()).
		Msg("Validating against ValueSet")

	fieldType := field.Type().String()
	switch fieldType {
	case "fhir.Coding", "*fhir.Coding":
		return p.validateCoding(field, valueSetURL)
	case "fhir.CodeableConcept", "*fhir.CodeableConcept":
		return p.validateCodeableConcept(field, valueSetURL)
	default:
		return &FilterResult{Passed: false}, nil
	}
}

// validateCoding checks a Coding against a ValueSet
func (p *ProcessorService) validateCoding(field reflect.Value, valueSetURL string) (*FilterResult, error) {
	var coding *fhir.Coding
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return &FilterResult{Passed: false}, nil
		}
		coding = field.Interface().(*fhir.Coding)
	} else {
		coding = field.Addr().Interface().(*fhir.Coding)
	}

	result, err := p.valueSetCache.ValidateCode(valueSetURL, coding)
	if err != nil {
		return nil, err
	}

	return &FilterResult{Passed: result.Valid}, nil
}

// validateCodeableConcept checks a CodeableConcept against a ValueSet
func (p *ProcessorService) validateCodeableConcept(field reflect.Value, valueSetURL string) (*FilterResult, error) {
	concept := field.Interface().(*fhir.CodeableConcept)
	for _, coding := range concept.Coding {
		result, err := p.valueSetCache.ValidateCode(valueSetURL, &coding)
		if err != nil {
			continue
		}
		if result.Valid {
			return &FilterResult{Passed: true}, nil
		}
	}
	return &FilterResult{Passed: false}, nil
}

// checkTokenWithSystemCode validates a token with system and code
func (p *ProcessorService) checkTokenWithSystemCode(field reflect.Value, system, code string) (*FilterResult, error) {
	fieldType := field.Type().String()
	switch fieldType {
	case "fhir.Coding", "*fhir.Coding":
		return p.checkCodingToken(field, system, code)
	case "fhir.CodeableConcept", "*fhir.CodeableConcept":
		return p.checkCodeableConceptToken(field, system, code)
	case "fhir.Identifier", "*fhir.Identifier":
		return p.checkIdentifierToken(field, system, code)
	default:
		if field.Kind() == reflect.String {
			return &FilterResult{Passed: field.String() == code}, nil
		}
		return &FilterResult{Passed: false}, nil
	}
}

// checkCodingToken validates a Coding against system and code
func (p *ProcessorService) checkCodingToken(field reflect.Value, system, code string) (*FilterResult, error) {
	var coding *fhir.Coding
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return &FilterResult{Passed: false}, nil
		}
		coding = field.Interface().(*fhir.Coding)
	} else {
		coding = field.Addr().Interface().(*fhir.Coding)
	}

	// Check system if provided
	if system != "" {
		if coding.System == nil || *coding.System != system {
			return &FilterResult{Passed: false}, nil
		}
	}

	// Check code
	if coding.Code == nil || *coding.Code != code {
		return &FilterResult{Passed: false}, nil
	}

	return &FilterResult{Passed: true}, nil
}

// checkCodeableConceptToken validates a CodeableConcept against system and code
func (p *ProcessorService) checkCodeableConceptToken(field reflect.Value, system, code string) (*FilterResult, error) {
	concept := field.Interface().(*fhir.CodeableConcept)
	for _, coding := range concept.Coding {
		codingField := reflect.ValueOf(coding)
		result, err := p.checkCodingToken(codingField, system, code)
		if err != nil {
			continue
		}
		if result.Passed {
			return result, nil
		}
	}
	return &FilterResult{Passed: false}, nil
}

// checkIdentifierToken validates an Identifier against system and code
func (p *ProcessorService) checkIdentifierToken(field reflect.Value, system, code string) (*FilterResult, error) {
	identifier := field.Interface().(*fhir.Identifier)

	// Check system if provided
	if system != "" {
		if identifier.System == nil || *identifier.System != system {
			return &FilterResult{Passed: false}, nil
		}
	}

	// Check value (code)
	if identifier.Value == nil || *identifier.Value != code {
		return &FilterResult{Passed: false}, nil
	}

	return &FilterResult{Passed: true}, nil
}

// checkDateFilter validates date fields
func (p *ProcessorService) checkDateFilter(field reflect.Value, param SearchParameter) (*FilterResult, error) {
	if field.Type().String() != "fhir.Date" {
		return &FilterResult{Passed: false}, nil
	}

	date := field.Interface().(fhir.Date)
	filterDate, err := time.Parse("2006-01-02", param.Value)
	if err != nil {
		return nil, err
	}

	fieldDate := date.Time
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
	default:
		return nil, fmt.Errorf("unsupported date comparator: %s", param.Comparator)
	}

	return &FilterResult{Passed: passed}, nil
}

// Helper functions
func parseTokenValue(value string) (string, string) {
	parts := strings.Split(value, "|")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}
