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

	// Log the start of the process
	fmt.Printf("Starting to populate and filter resource of type: %s\n", resourceType)

	// Process each field
	for i := 0; i < resourceValue.NumField(); i++ {
		field := resourceValue.Field(i)
		fieldName := resourceValue.Type().Field(i).Name
		path := fmt.Sprintf("%s.%s", resourceType, strings.ToLower(fieldName))
		fmt.Printf("Processing path: %s\n", path)

		// Skip if already processed
		if _, processed := p.processedPaths.Load(path); processed {
			fmt.Printf("Skipping already processed path: %s\n", path)
			continue
		}

		// Mark as processed
		p.processedPaths.Store(path, true)

		// Get rows for this path
		rows, exists := result[path]
		if !exists {
			fmt.Printf("No data found for path: %s\n", path)
			continue
		}

		// Populate the field
		fmt.Printf("Populating field: %s\n", fieldName)
		if err := p.populateField(ctx, field, path, rows); err != nil {
			fmt.Printf("Error populating field %s: %v\n", fieldName, err)
			return false, err
		}

		// Apply filter if needed
		if filter != nil {
			searchType, err := p.pathInfoSvc.GetSearchTypeByCode(path, filter.Code)
			if err == nil { // Found a search parameter for this path
				fmt.Printf("Applying filter on field: %s\n", fieldName)
				passed, err := p.checkFilter(ctx, field, path, searchType, filter)
				if err != nil {
					fmt.Printf("Error applying filter on field %s: %v\n", fieldName, err)
					return false, err
				}
				if !passed {
					fmt.Printf("Field %s did not pass the filter\n", fieldName)
					return false, nil
				}
			}
		}
	}

	// Log the end of the process
	fmt.Printf("Finished populating and filtering resource of type: %s\n", resourceType)
	return true, nil
}
func (p *ProcessorService) populateField(ctx context.Context, field reflect.Value, path string, rows []datasource.RowData) error {
	if !field.CanSet() {
		fmt.Printf("Field at path %s cannot be set\n", path)
		return nil
	}

	switch field.Kind() {
	case reflect.Ptr:
		if field.IsNil() {
			fmt.Printf("Initializing nil pointer for field at path %s\n", path)
			field.Set(reflect.New(field.Type().Elem()))
		}
		fmt.Printf("Populating pointer field at path %s\n", path)
		return p.populateField(ctx, field.Elem(), path, rows)

	case reflect.Slice:
		fmt.Printf("Populating slice field at path %s\n", path)
		return p.populateSlice(ctx, field, path, rows)

	case reflect.Struct:
		fmt.Printf("Populating struct field at path %s\n", path)
		return p.populateStruct(ctx, field, path, rows)

	default:
		fmt.Printf("Populating basic field at path %s\n", path)
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
		fmt.Printf("No rows to populate struct field at path %s\n", path)
		return nil
	}

	row := rows[0] // Take first row for struct
	for key, value := range row.Data {
		// Uppercase first letter of the key
		upperKey := strings.ToUpper(key[:1]) + key[1:]

		structField := field.FieldByName(upperKey)
		if structField.IsValid() && structField.CanSet() {
			fmt.Printf("Populating struct field %s with key %s and value %v\n", path, upperKey, value)
			if err := p.setField(ctx, structField, value); err != nil {
				fmt.Printf("Error setting field %s: %v\n", upperKey, err)
				return err
			}
		} else {
			fmt.Printf("Field %s is not valid or cannot be set\n", upperKey)
			if !structField.IsValid() {
				fmt.Printf("Reason: Field %s is not valid\n", upperKey)
			}
			if !structField.CanSet() {
				fmt.Printf("Reason: Field %s cannot be set\n", upperKey)
			}
		}
	}

	return nil
}

func (p *ProcessorService) populateBasicField(field reflect.Value, rows []datasource.RowData) error {
	if len(rows) == 0 {
		fmt.Printf("No rows to populate basic field %s\n", field.Type().Name())
		return nil
	}

	row := rows[0] // Take first row for basic field
	for _, value := range row.Data {
		fmt.Printf("Populating basic field %s with value %v\n", field.Type().Name(), value)
		return p.setField(context.Background(), field, value)
	}

	return nil
}

// setField handles setting field values with appropriate type conversion
func (p *ProcessorService) setField(ctx context.Context, field reflect.Value, value interface{}) error {
	if !field.CanSet() {
		fmt.Printf("Field %s cannot be set\n", field.Type().Name())
		return nil
	}

	if value == nil {
		fmt.Printf("Setting field %s to zero value\n", field.Type().Name())
		field.Set(reflect.Zero(field.Type()))
		return nil
	}

	// Handle pointer types
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			fmt.Printf("Initializing nil pointer for field %s\n", field.Type().Elem().Name())
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	// Check for special types
	switch field.Type().String() {
	case "fhir.Date":
		fmt.Printf("Setting field %s as fhir.Date\n", field.Type().Name())
		return p.setDateField(field, value)
	case "json.Number":
		fmt.Printf("Setting field %s as json.Number\n", field.Type().Name())
		return p.setJSONNumber(field, value)
	}

	// Handle Code types (fields with Code method)
	if typeHasCodeMethod(field.Type()) {
		fmt.Printf("Setting Code field %s with value %v\n", field.Type().Name(), value)

		// Convert the input value to a string
		var strValue string
		switch v := value.(type) {
		case string:
			strValue = v
		case []uint8:
			strValue = string(v)
		case int, int64, float64:
			return fmt.Errorf("numeric value %v not supported for code field %s, expected string code", v, field.Type().Name())
		default:
			return fmt.Errorf("unsupported type for code value: %T", value)
		}

		// Create a new instance of the field type
		codePtr := reflect.New(field.Type()).Interface()

		// Use the UnmarshalJSON method to set the value
		jsonValue, err := json.Marshal(strValue)
		if err != nil {
			return fmt.Errorf("failed to marshal code string: %w", err)
		}

		if unmarshaler, ok := codePtr.(json.Unmarshaler); ok {
			if err := unmarshaler.UnmarshalJSON(jsonValue); err != nil {
				return fmt.Errorf("failed to unmarshal code value '%s': %w", strValue, err)
			}
			// Set the field value from the pointer
			field.Set(reflect.ValueOf(codePtr).Elem())
			return nil
		}

		return fmt.Errorf("code type %s does not implement UnmarshalJSON", field.Type().Name())
	}

	fmt.Printf("Setting basic field %s\n", field.Type().Name())
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
