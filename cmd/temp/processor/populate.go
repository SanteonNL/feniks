package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/SanteonNL/fenix/models/fhir"
)

// setField handles setting field values with appropriate type conversion
func (p *ProcessorService) setField(ctx context.Context, structPath string, structPtr interface{}, fieldName string, value interface{}) error {
	structValue := reflect.ValueOf(structPtr)
	if structValue.Kind() != reflect.Ptr || structValue.IsNil() {
		return fmt.Errorf("structPtr must be a non-nil pointer")
	}

	structElem := structValue.Elem()
	field := structElem.FieldByName(fieldName)
	if !field.IsValid() || !field.CanSet() {
		return fmt.Errorf("invalid or cannot set field: %s", fieldName)
	}

	if value == nil {
		field.Set(reflect.Zero(field.Type()))
		return nil
	}

	// Handle pointer types
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	// Check for special types
	switch field.Type().String() {
	case "fhir.Date":
		return p.setDateField(field, value)
	case "json.Number":
		return p.setJSONNumber(field, value)
	}

	// Handle concept mapping for code types
	if typeHasCodeMethod(field.Type()) {
		if mappedValue, _, err := p.performConceptMapping(ctx, structPath, fmt.Sprint(value), true); err != nil {
			return err
		} else {
			value = mappedValue
		}
	}

	return p.setBasicField(field, value)
}

// setDateField handles FHIR Date type
func (p *ProcessorService) setDateField(field reflect.Value, value interface{}) error {
	if !field.CanAddr() {
		return fmt.Errorf("cannot take address of date field")
	}

	dateStr := ""
	switch v := value.(type) {
	case string:
		dateStr = v
	case []uint8:
		dateStr = string(v)
	default:
		return fmt.Errorf("cannot convert %T to Date", value)
	}

	date := field.Addr().Interface().(*fhir.Date)
	if err := date.UnmarshalJSON([]byte(`"` + dateStr + `"`)); err != nil {
		return fmt.Errorf("failed to parse date: %w", err)
	}

	return nil
}

// setJSONNumber handles json.Number type
func (p *ProcessorService) setJSONNumber(field reflect.Value, value interface{}) error {
	var num json.Number
	switch v := value.(type) {
	case json.Number:
		num = v
	case string:
		num = json.Number(v)
	case float64:
		num = json.Number(strconv.FormatFloat(v, 'f', -1, 64))
	case int64:
		num = json.Number(strconv.FormatInt(v, 10))
	case []uint8:
		num = json.Number(string(v))
	default:
		return fmt.Errorf("cannot convert %T to json.Number", value)
	}

	field.Set(reflect.ValueOf(num))
	return nil
}

// setBasicField handles basic field types
func (p *ProcessorService) setBasicField(field reflect.Value, value interface{}) error {
	v := reflect.ValueOf(value)

	// Direct set if types match
	if field.Type() == v.Type() {
		field.Set(v)
		return nil
	}

	// Type conversion
	switch field.Kind() {
	case reflect.String:
		field.SetString(fmt.Sprint(value))
	case reflect.Bool:
		bVal, err := strconv.ParseBool(fmt.Sprint(value))
		if err != nil {
			return err
		}
		field.SetBool(bVal)
	case reflect.Int, reflect.Int64:
		iVal, err := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(iVal)
	case reflect.Float64:
		fVal, err := strconv.ParseFloat(fmt.Sprint(value), 64)
		if err != nil {
			return err
		}
		field.SetFloat(fVal)
	default:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	}

	return nil
}

// Helper function to check if type has Code method
func typeHasCodeMethod(t reflect.Type) bool {
	_, ok := t.MethodByName("Code")
	return ok
}
