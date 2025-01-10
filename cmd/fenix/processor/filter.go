package processor

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/SanteonNL/fenix/cmd/fenix/types"
	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog/log"
)

func (p *ProcessorService) checkFilter(field reflect.Value, path string, filters []*types.Filter) (bool, error) {
	// If no filters, return true
	if len(filters) == 0 {
		return true, nil
	}

	// All filters must pass for the field to pass
	for _, filter := range filters {
		// Get search type from search parameter service
		searchType, exists := p.pathInfoSvc.GetSearchTypeByPathAndCode(path, filter.Code)
		if !exists {
			p.log.Debug().
				Str("path", path).
				Str("filterCode", filter.Code).
				Msg("No search type found for filter")
			return true, nil
		}

		p.log.Debug().
			Str("path", path).
			Str("searchType", searchType).
			Str("filterCode", filter.Code).
			Str("filterValue", filter.Value).
			Str("fieldValue", getCodeFromField(field)).
			Msg("Checking filter")

		passed, err := p.checkSingleFilter(field, path, searchType, filter)
		if err != nil {
			return false, err
		}
		if !passed {
			return false, nil
		}
	}

	return true, nil
}

func (p *ProcessorService) checkSingleFilter(field reflect.Value, path string, searchType string, filter *types.Filter) (bool, error) {
	switch searchType {
	case "token":
		return p.checkTokenFilter(field, path, filter.Value)
	case "string":
		return p.checkStringFilter(field, filter.Value)
	case "date":
		return p.checkDateFilter(field, filter.Value)
	// case "number":
	// 	return p.checkNumberFilter(field, filter.Value)
	// case "reference":
	// 	return p.checkReferenceFilter(field, filter.Value)
	// case "quantity":
	// 	return p.checkQuantityFilter(field, filter.Value)
	default:
		p.log.Debug().
			Str("type", searchType).
			Msg("Unsupported search parameter type")
		return true, nil
	}
}

func (p *ProcessorService) checkTokenFilter(field reflect.Value, path string, filterValue string) (bool, error) {
	// Get path info to check for ValueSet
	pathInfo, err := p.pathInfoSvc.GetPathInfo(path)
	if err == nil && pathInfo.ValueSet != "" {
		return p.checkValueSetFilter(field, pathInfo.ValueSet)
	}

	// If no ValueSet, do direct comparison
	code := getCodeFromField(field)
	return code == filterValue, nil
}

func (p *ProcessorService) checkValueSetFilter(field reflect.Value, valueSetURL string) (bool, error) {
	p.log.Debug().
		Str("valueSetURL", valueSetURL).
		Msg("Checking ValueSet filter")

	switch field.Type().String() {
	case "fhir.Coding", "*fhir.Coding":
		coding := getCodingFromField(field)
		if coding == nil {
			return false, nil
		}
		valid, err := p.valueSetSvc.ValidateCode(valueSetURL, coding)
		if err != nil {
			return false, err
		}
		return valid.Valid, nil

	case "fhir.CodeableConcept", "*fhir.CodeableConcept":
		concept := getCodeableConceptFromField(field)
		if concept == nil {
			return false, nil
		}
		for _, coding := range concept.Coding {
			valid, err := p.valueSetSvc.ValidateCode(valueSetURL, &coding)
			if err == nil && valid.Valid {
				return true, nil
			}
		}
		return false, nil

	default:
		return false, fmt.Errorf("unsupported field type for ValueSet validation: %s", field.Type().String())
	}
}

// Helper functions

func getCodeFromField(field reflect.Value) string {
	log.Debug().Str("fieldType", field.Type().String()).Msg("Getting code from field")

	// Helper function to check if type has Code method)
	if typeHasCodeMethod(field.Type()) {
		return field.MethodByName("Code").Call([]reflect.Value{})[0].String()
	}

	switch field.Type().String() {
	case "fhir.Coding", "*fhir.Coding":
		coding := getCodingFromField(field)
		if coding != nil && coding.Code != nil {
			return *coding.Code
		}
	case "fhir.CodeableConcept", "*fhir.CodeableConcept":
		concept := getCodeableConceptFromField(field)
		if concept != nil && len(concept.Coding) > 0 && concept.Coding[0].Code != nil {
			return *concept.Coding[0].Code
		}
	case "string", "*string":
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				return ""
			}
			return field.Elem().String()
		}
		return field.String()
	}
	return ""
}

func getCodingFromField(field reflect.Value) *fhir.Coding {
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil
		}
		return field.Interface().(*fhir.Coding)
	}
	return field.Addr().Interface().(*fhir.Coding)
}

func getCodeableConceptFromField(field reflect.Value) *fhir.CodeableConcept {
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil
		}
		return field.Interface().(*fhir.CodeableConcept)
	}
	return field.Addr().Interface().(*fhir.CodeableConcept)
}

// Basic type filters

func (p *ProcessorService) checkStringFilter(field reflect.Value, filterValue string) (bool, error) {
	if field.Kind() == reflect.Ptr && !field.IsNil() {
		field = field.Elem()
	}

	if field.Kind() != reflect.String {
		return false, fmt.Errorf("field is not a string")
	}

	return strings.Contains(strings.ToLower(field.String()),
		strings.ToLower(filterValue)), nil
}

func (p *ProcessorService) checkDateFilter(field reflect.Value, filterValue string) (bool, error) {
	// Basic date comparison - can be expanded based on requirements
	if field.Kind() == reflect.Ptr && !field.IsNil() {
		field = field.Elem()
	}

	date, ok := field.Interface().(fhir.Date)
	if !ok {
		return false, fmt.Errorf("field is not a date")
	}

	return date.String() == filterValue, nil
}
