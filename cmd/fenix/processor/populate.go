package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/SanteonNL/fenix/cmd/fenix/datasource"
	"github.com/SanteonNL/fenix/models/fhir"
)

func (p *ProcessorService) populateAndFilter(ctx context.Context, resource interface{}, result datasource.ResourceResult, filter *Filter) (bool, error) {
	resourceValue := reflect.ValueOf(resource).Elem()
	resourceType := resourceValue.Type().Name()

	// Process each field
	for i := 0; i < resourceValue.NumField(); i++ {
		field := resourceValue.Field(i)
		fieldName := resourceValue.Type().Field(i).Name
		path := fmt.Sprintf("%s.%s", resourceType, strings.ToLower(fieldName))

		// Skip if already processed
		if _, processed := p.processedPaths.Load(path); processed {
			continue
		}

		// Mark as processed
		p.processedPaths.Store(path, true)

		// Get rows for this path
		rows, exists := result[path]
		if !exists {
			continue
		}

		// Populate the field
		if err := p.populateField(ctx, field, path, rows); err != nil {
			return false, err
		}

		// Apply filter if needed
		if filter != nil {
			searchType, err := p.pathInfoSvc.GetSearchTypeByCode(path, filter.Code)
			if err == nil { // Found a search parameter for this path
				passed, err := p.checkFilter(ctx, field, path, searchType, filter)
				if err != nil {
					return false, err
				}
				if !passed {
					return false, nil
				}
			}
		}
	}

	return true, nil
}

func (p *ProcessorService) populateField(ctx context.Context, field reflect.Value, path string, rows []datasource.RowData) error {
	if !field.CanSet() {
		return nil
	}

	switch field.Kind() {
	case reflect.Ptr:
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return p.populateField(ctx, field.Elem(), path, rows)

	case reflect.Slice:
		return p.populateSlice(ctx, field, path, rows)

	case reflect.Struct:
		return p.populateStruct(ctx, field, path, rows)

	default:
		return p.populateBasicField(field, rows)
	}
}

func (p *ProcessorService) populateSlice(ctx context.Context, field reflect.Value, path string, rows []datasource.RowData) error {
	sliceType := field.Type()
	newSlice := reflect.MakeSlice(sliceType, 0, len(rows))

	for _, row := range rows {
		elem := reflect.New(sliceType.Elem()).Elem()
		if err := p.populateField(ctx, elem, path, []datasource.RowData{row}); err != nil {
			return err
		}
		newSlice = reflect.Append(newSlice, elem)
	}

	field.Set(newSlice)
	return nil
}

func (p *ProcessorService) populateStruct(ctx context.Context, field reflect.Value, path string, rows []datasource.RowData) error {
	if len(rows) == 0 {
		return nil
	}

	row := rows[0] // Take first row for struct
	for key, value := range row.Data {
		field := field.FieldByName(key)
		if field.IsValid() && field.CanSet() {
			if err := p.setField(ctx, field, value); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *ProcessorService) populateBasicField(field reflect.Value, rows []datasource.RowData) error {
	if len(rows) == 0 {
		return nil
	}

	row := rows[0] // Take first row for basic field
	for _, value := range row.Data {
		return p.setField(context.Background(), field, value)
	}

	return nil
}

// setField handles setting field values with appropriate type conversion
func (p *ProcessorService) setField(ctx context.Context, field reflect.Value, value interface{}) error {
	if !field.CanSet() {
		return nil
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

		if mappedValue, err := p.conceptMapSvc.TranslateCode(nil, field.Type().String(), false); err != nil {
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
