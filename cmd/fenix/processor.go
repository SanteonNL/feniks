package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type SearchParameterMap map[string]SearchParameter

type SearchParameter struct {
	Code       string   `json:"code"`
	Type       string   `json:"type"`
	Modifier   []string `json:"modifier,omitempty"`
	Comparator string   `json:"comparator,omitempty"`
	Value      string   `json:"value"`
}

type ResourceProcessor struct {
	resourceType string
	searchParams SearchParameterMap
	log          zerolog.Logger
}

type FilterResult struct {
	Passed  bool
	Message string
}

func NewResourceProcessor(resourceType string, searchParams SearchParameterMap, log zerolog.Logger) *ResourceProcessor {
	return &ResourceProcessor{
		resourceType: resourceType,
		searchParams: searchParams,
		log:          log,
	}
}

func ProcessResources(ds *DataSource, patientID string, searchParams SearchParameterMap, log zerolog.Logger) ([]interface{}, error) {
	// Read all resources
	results, err := ds.ReadResources(patientID)
	if err != nil {
		return nil, fmt.Errorf("error reading data: %w", err)
	}

	log.Info().Msgf("Number of results found: %d", len(results))

	processor := NewResourceProcessor(ds.resourceType, searchParams, log)
	var processedResources []interface{}

	// Process each resource result
	for _, result := range results {
		log.Debug().Interface("result %v", result).Msg("Processing resource result")
		// Create new resource instance
		resource, err := CreateResource(ds.resourceType)
		if err != nil {
			return nil, fmt.Errorf("error creating resource: %w", err)
		}

		resourceValue := reflect.ValueOf(resource).Elem()

		// Populate and filter the resource
		filterResult, err := processor.populateResourceStruct(resourceValue, result)
		if err != nil {
			// Log error but continue with other resources
			continue
		}

		if !filterResult.Passed {
			continue
		}

		processedResources = append(processedResources, resource)
	}

	return processedResources, nil
}

// populateResourceStruct maintains your current population logic
func (rp *ResourceProcessor) populateResourceStruct(value reflect.Value, result ResourceResult) (*FilterResult, error) {
	return rp.determinePopulateType(rp.resourceType, value, "", result)
}

// determinePopulateType handles different field types
func (rp *ResourceProcessor) determinePopulateType(structPath string, value reflect.Value, parentID string, result ResourceResult) (*FilterResult, error) {
	rp.log.Debug().Str("structPath", structPath).Str("value.Kind()", value.Kind().String()).Msg("Determining populate type")

	rows, exists := result[structPath]
	if !exists {
		return &FilterResult{Passed: true}, nil
	}

	switch value.Kind() {
	case reflect.Slice:
		return rp.populateSlice(structPath, value, parentID, rows, result)
	case reflect.Struct:
		return rp.populateStruct(structPath, value, parentID, rows, result)
	case reflect.Ptr:
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return rp.determinePopulateType(structPath, value.Elem(), parentID, result)
	default:
		return rp.setBasicType(structPath, value, parentID, rows)
	}
}

// Keeping your current slice logic which is important for filtering
func (rp *ResourceProcessor) populateSlice(structPath string, value reflect.Value, parentID string, rows []RowData, result ResourceResult) (*FilterResult, error) {
	filteredSlice := reflect.MakeSlice(value.Type(), 0, len(rows))
	anyPassed := false

	for _, row := range rows {
		if row.ParentID == parentID || row.ParentID == "" {
			valueElement := reflect.New(value.Type().Elem()).Elem()

			// Populate the element and check filters
			structFilterResult, err := rp.populateStructAndNestedFields(structPath, valueElement, row, result)
			if err != nil {
				return nil, err
			}

			if structFilterResult.Passed {
				anyPassed = true
				filteredSlice = reflect.Append(filteredSlice, valueElement)
			}
		}
	}

	// If any element passed, keep all elements
	if anyPassed {
		value.Set(filteredSlice)
		return &FilterResult{Passed: true}, nil
	}

	return &FilterResult{
		Passed:  false,
		Message: fmt.Sprintf("No elements in slice passed filters at %s", structPath),
	}, nil
}

// populateStruct handles populating struct fields including nested structures
func (rp *ResourceProcessor) populateStruct(path string, value reflect.Value, parentID string, rows []RowData, result ResourceResult) (*FilterResult, error) {
	// Process each row that matches the parent ID
	for _, row := range rows {
		if row.ParentID == parentID || parentID == "" {
			log.Debug().Str("path", path).Str("row.ID", row.ID).Msg("Processing struct")
			// First populate direct fields
			filterResult, err := rp.populateStructFields(path, value.Addr().Interface(), row)
			if err != nil {
				return nil, fmt.Errorf("failed to populate struct fields at %s: %w", path, err)
			}

			// If direct fields were filtered out, stop processing
			if !filterResult.Passed {
				return filterResult, nil
			}

			// Then handle nested fields
			nestedResult, err := rp.populateNestedFields(path, value, result, row.ID)
			if err != nil {
				return nil, err
			}

			// Return immediately if nested fields pass
			if nestedResult.Passed {
				return nestedResult, nil
			}
		}
	}

	// If we get here and haven't found any matching rows, still consider it a pass
	return &FilterResult{
		Passed:  true,
		Message: fmt.Sprintf("No matching rows for struct at %s", path),
	}, nil
}

// Part 1: Struct and Nested Fields
func (rp *ResourceProcessor) populateStructAndNestedFields(structPath string, value reflect.Value, row RowData, result ResourceResult) (*FilterResult, error) {
	// First populate and filter struct fields
	structResult, err := rp.populateStructFields(structPath, value.Addr().Interface(), row)
	if err != nil {
		return nil, fmt.Errorf("failed to populate struct fields at %s: %w", structPath, err)
	}

	if !structResult.Passed {
		return structResult, nil
	}

	// Then handle nested fields
	nestedResult, err := rp.populateNestedFields(structPath, value, result, row.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to populate nested fields at %s: %w", structPath, err)
	}

	return nestedResult, nil
}

func (rp *ResourceProcessor) populateNestedFields(parentPath string, parentValue reflect.Value, result ResourceResult, parentID string) (*FilterResult, error) {
	for i := 0; i < parentValue.NumField(); i++ {
		field := parentValue.Field(i)
		fieldName := parentValue.Type().Field(i).Name
		fieldPath := fmt.Sprintf("%s.%s", parentPath, strings.ToLower(fieldName))

		if hasDataForPath(result, fieldPath) {
			filterResult, err := rp.determinePopulateType(fieldPath, field, parentID, result)
			if err != nil {
				return nil, err
			}
			if !filterResult.Passed {
				return filterResult, nil
			}
		}
	}

	return &FilterResult{Passed: true}, nil
}

func (rp *ResourceProcessor) populateStructFields(structPath string, structPtr interface{}, row RowData) (*FilterResult, error) {
	structValue := reflect.ValueOf(structPtr).Elem()
	structType := structValue.Type()

	fieldsPopulated := false

	// Process regular fields
	for key, value := range row.Data {
		for i := 0; i < structType.NumField(); i++ {
			fieldName := structType.Field(i).Name
			if strings.EqualFold(fieldName, key) {
				rp.log.Debug().Str("structPath", structPath).Str("fieldName", fieldName).Interface("value", value).Msg("Setting field")
				if err := rp.setField(structPath, structPtr, fieldName, value); err != nil {
					rp.log.Error().Err(err).Str("structPath", structPath).Str("fieldName", fieldName).Msg("Failed to set field")
					return nil, err
				}
				fieldsPopulated = true

				// Check field filter
				fieldPath := fmt.Sprintf("%s.%s", structPath, strings.ToLower(fieldName))
				if filterResult, err := rp.checkFilter(fieldPath, structValue.Field(i)); err != nil {
					rp.log.Error().Err(err).Str("fieldPath", fieldPath).Msg("Failed to check filter")
					return nil, err
				} else if !filterResult.Passed {
					rp.log.Debug().Str("fieldPath", fieldPath).Msg("Field did not pass filter")
					return filterResult, nil
				}
			}
		}
	}

	// Handle ID field
	if idField := structValue.FieldByName("Id"); idField.IsValid() && idField.CanSet() {
		rp.log.Debug().Str("structPath", structPath).Str("fieldName", "Id").Str("value", row.ID).Msg("Setting ID field")
		if err := rp.setField(structPath, structPtr, "Id", row.ID); err != nil {
			rp.log.Error().Err(err).Str("structPath", structPath).Str("fieldName", "Id").Msg("Failed to set ID field")
			return nil, err
		}
		fieldsPopulated = true
	}

	if !fieldsPopulated {
		rp.log.Debug().Str("structPath", structPath).Msg("No fields populated")
		return &FilterResult{
			Passed:  true,
			Message: fmt.Sprintf("No fields populated for %s", structPath),
		}, nil
	}

	// rp.log.Debug().Str("structPath", structPath).Msg("All fields populated successfully")
	return &FilterResult{Passed: true}, nil
}

// Part 2: Field Setting and Type Conversion
func (rp *ResourceProcessor) setField(structPath string, structPtr interface{}, fieldName string, value interface{}) error {

	structValue := reflect.ValueOf(structPtr)
	if structValue.Kind() != reflect.Ptr || structValue.IsNil() {
		return fmt.Errorf("structPtr must be a non-nil pointer to struct")
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

	// Handle pointer types first - initialize if needed
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem() // Dereference for further processing
	}

	rp.log.Debug().Str("structPath", structPath).Str("fieldName", fieldName).Str("fieldType", field.Type().String()).Interface("value", value).Msg("Setting field")

	// Now check for special types after potentially dereferencing
	switch field.Type().String() {
	case "fhir.Date":
		return rp.setDateField(field, value)
	case "json.Number":
		return rp.setJSONNumber(field, value)
	}

	// Check if type implements UnmarshalJSON
	if unmarshaler, ok := field.Addr().Interface().(json.Unmarshaler); ok {
		rp.log.Debug().Str("field", field.Type().String()).Msg("Setting field with UnmarshalJSON")
		var jsonBytes []byte
		var err error

		switch v := value.(type) {
		case string:
			jsonBytes = []byte(`"` + v + `"`)
		case []byte:
			jsonBytes = v
		default:
			if jsonBytes, err = json.Marshal(value); err != nil {
				return fmt.Errorf("failed to marshal value: %w", err)
			}
		}

		if err := unmarshaler.UnmarshalJSON(jsonBytes); err != nil {
			return fmt.Errorf("failed to unmarshal value for type %s: %w", field.Type().String(), err)
		}
		return nil
	}

	// Handle basic types
	return rp.setBasicField(field, value)
}

func (rp *ResourceProcessor) setDateField(field reflect.Value, value interface{}) error {
	rp.log.Debug().Str("field", field.Type().String()).Msg("Setting date field")
	// Ensure we can take the address of the field
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

	// Get the Date object we can unmarshal into
	date := field.Addr().Interface().(*fhir.Date)
	if err := date.UnmarshalJSON([]byte(`"` + dateStr + `"`)); err != nil {
		return fmt.Errorf("failed to parse date: %w", err)
	}

	return nil
}

func (rp *ResourceProcessor) setJSONNumber(field reflect.Value, value interface{}) error {
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

func (rp *ResourceProcessor) setBasicField(field reflect.Value, value interface{}) error {
	v := reflect.ValueOf(value)
	if field.Type() == v.Type() {
		field.Set(v)
		return nil
	}

	// Handle type conversions
	switch field.Kind() {
	case reflect.String:
		field.SetString(fmt.Sprint(value))
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(fmt.Sprint(value))
		if err != nil {
			return err
		}
		field.SetBool(boolVal)
	case reflect.Int, reflect.Int64:
		intVal, err := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(intVal)
	case reflect.Float64:
		floatVal, err := strconv.ParseFloat(fmt.Sprint(value), 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)
	default:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	}
	return nil
}

// Helper function to check for nested data
func hasDataForPath(resultMap map[string][]RowData, path string) bool {
	if _, exists := resultMap[path]; exists {
		return true
	}
	return false
}

// Helper function to get byte value
func getByteValue(v interface{}) ([]byte, error) {
	switch value := v.(type) {
	case string:
		return []byte(value), nil
	case []byte:
		return value, nil
	default:
		return json.Marshal(v)
	}
}

func (rp *ResourceProcessor) setBasicType(path string, field reflect.Value, parentID string, rows []RowData) (*FilterResult, error) {
	for _, row := range rows {
		if row.ParentID == parentID || parentID == "" {
			for key, value := range row.Data {
				if err := rp.setField(path, field.Addr().Interface(), key, value); err != nil {
					return nil, err
				}

				// Check filter
				filterResult, err := rp.checkFilter(path, field)
				if err != nil {
					return nil, err
				}
				return filterResult, nil
			}
		}
	}
	return &FilterResult{Passed: true}, nil
}
