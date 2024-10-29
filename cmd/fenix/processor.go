package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
)

type SearchParameterMap map[string]SearchParameter

type SearchParameter struct {
	Code       string   `json:"code"`
	Type       string   `json:"type"`
	Modifier   []string `json:"modifier,omitempty"`
	Comparator string   `json:"comparator,omitempty"`
	Value      string   `json:"value"`
}

// Add this type at the top level
type ProcessedPaths map[string]bool

// Modify ResourceProcessor to track processed paths
type ResourceProcessor struct {
	resourceType   string
	searchParams   SearchParameterMap
	log            zerolog.Logger
	processedPaths ProcessedPaths // Add this field
}

type FilterResult struct {
	Passed  bool
	Message string
}

func NewResourceProcessor(resourceType string, searchParams SearchParameterMap, log zerolog.Logger) *ResourceProcessor {
	return &ResourceProcessor{
		resourceType:   resourceType,
		searchParams:   searchParams,
		log:            log,
		processedPaths: make(ProcessedPaths), // Initialize the map
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

	outputDir := "output/temp"
	if err := WriteToJSON(results, "raw_results", outputDir, log); err != nil {
		log.Error().Err(err).Msg("Failed to write raw results")
		// Continue processing despite write error
	}

	// Process each resource result
	for _, result := range results {
		log.Debug().Interface("result", result).Msg("Processing resource result")
		// Create new resource instance
		resource, err := CreateResource(ds.resourceType)
		if err != nil {
			return nil, fmt.Errorf("error creating resource: %w", err)
		}

		resourceValue := reflect.ValueOf(resource).Elem()

		// Populate and filter the resource
		filterResult, err := processor.populateResourceStruct(resourceValue, result)
		if err != nil {
			log.Error().Err(err).Msg("Error processing resource")
			continue
		}

		// Skip the entire resource if any filter failed
		if !filterResult.Passed {
			log.Debug().Str("resourceType", ds.resourceType).Msg("Resource filtered out")
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

// Modify populateSlice to mark processed paths
func (rp *ResourceProcessor) populateSlice(structPath string, value reflect.Value, parentID string, rows []RowData, result ResourceResult) (*FilterResult, error) {
	rp.log.Debug().
		Str("structPath", structPath).
		Msg("Populating slice")

	// Mark this path as processed
	rp.processedPaths[structPath] = true

	// Create slice to hold all elements
	allElements := reflect.MakeSlice(value.Type(), 0, len(rows))
	anyElementPassed := false

	// Check if there's a filter for this slice path
	hasFilter := false
	if _, exists := rp.searchParams[structPath]; exists {
		hasFilter = true
		rp.log.Debug().
			Str("structPath", structPath).
			Msg("Found filter for slice")
	}

	for _, row := range rows {
		if row.ParentID == parentID || row.ParentID == "" {
			valueElement := reflect.New(value.Type().Elem()).Elem()

			// Populate the element
			filterResult, err := rp.populateStructAndNestedFields(structPath, valueElement, row, result)
			if err != nil {
				return nil, fmt.Errorf("error populating slice element: %w", err)
			}

			// If this path has a filter, check if any element passes
			if hasFilter && filterResult.Passed {
				elementFilterResult, err := rp.checkFilter(structPath, valueElement)
				if err != nil {
					return nil, fmt.Errorf("error checking filter for slice element: %w", err)
				}
				if elementFilterResult.Passed {
					anyElementPassed = true
				}
			} else if !hasFilter {
				// If no filter exists, consider it passed
				anyElementPassed = true
			}

			// Always add element to the slice
			allElements = reflect.Append(allElements, valueElement)
		}
	}

	// If we have a filter and no elements passed, return filter failure
	if hasFilter && !anyElementPassed {
		return &FilterResult{
			Passed:  false,
			Message: fmt.Sprintf("No elements in slice at %s passed filters", structPath),
		}, nil
	}

	// Set the complete slice with all elements
	value.Set(allElements)
	return &FilterResult{
		Passed:  true,
		Message: fmt.Sprintf("Slice at %s processed successfully", structPath),
	}, nil
}

// populateStruct handles populating struct fields including nested structures
// Update populateStruct to properly handle filter failures
func (rp *ResourceProcessor) populateStruct(path string, value reflect.Value, parentID string, rows []RowData, result ResourceResult) (*FilterResult, error) {
	// First check if there's a filter for this struct level
	rp.log.Debug().Str("path", path).Msg("Checking struct level filter")

	if filterResult, err := rp.checkFilter(path, value); err != nil {
		return nil, fmt.Errorf("failed to check struct filter at %s: %w", path, err)
	} else if !filterResult.Passed {
		rp.log.Debug().Str("path", path).Msg("Struct level filter did not pass")
		return filterResult, nil
	}

	// Track if we populated any fields
	anyFieldsPopulated := false
	var lastFilterResult *FilterResult

	// Process each row that matches the parent ID
	for _, row := range rows {
		if row.ParentID == parentID || parentID == "" {
			rp.log.Debug().Str("path", path).Str("row.ID", row.ID).Msg("Processing struct")

			// First populate direct fields
			filterResult, err := rp.populateStructFields(path, value.Addr().Interface(), row, result)
			if err != nil {
				return nil, fmt.Errorf("failed to populate struct fields at %s: %w", path, err)
			}

			// If direct fields were filtered out, stop processing this row
			if !filterResult.Passed {
				return filterResult, nil
			}

			// Then handle nested fields
			nestedResult, err := rp.populateNestedFields(path, value, result, row.ID)
			if err != nil {
				return nil, err
			}

			// If nested fields were filtered out, stop processing this row
			if !nestedResult.Passed {
				return nestedResult, nil
			}

			anyFieldsPopulated = true
			lastFilterResult = nestedResult
		}
	}

	if !anyFieldsPopulated {
		return &FilterResult{
			Passed:  true,
			Message: fmt.Sprintf("No matching rows for struct at %s", path),
		}, nil
	}

	return lastFilterResult, nil
}

// Part 1: Struct and Nested Fields
func (rp *ResourceProcessor) populateStructAndNestedFields(structPath string, value reflect.Value, row RowData, result ResourceResult) (*FilterResult, error) {
	// First populate and filter struct fields
	structResult, err := rp.populateStructFields(structPath, value.Addr().Interface(), row, result)
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

// Modify populateNestedFields to check processed paths
func (rp *ResourceProcessor) populateNestedFields(parentPath string, parentValue reflect.Value, result ResourceResult, parentID string) (*FilterResult, error) {
	for i := 0; i < parentValue.NumField(); i++ {
		field := parentValue.Field(i)
		fieldName := parentValue.Type().Field(i).Name
		fieldPath := fmt.Sprintf("%s.%s", parentPath, strings.ToLower(fieldName))

		// // Skip if we've already processed this path
		// if rp.processedPaths[fieldPath] {
		// 	rp.log.Debug().
		// 		Str("fieldPath", fieldPath).
		// 		Msg("Skipping already processed nested field")
		// 	continue
		// }

		if hasDataForPath(result, fieldPath) {
			rp.processedPaths[fieldPath] = true
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

func (rp *ResourceProcessor) populateStructFields(structPath string, structPtr interface{}, row RowData, result ResourceResult) (*FilterResult, error) {
	structValue := reflect.ValueOf(structPtr).Elem()
	structType := structValue.Type()

	fieldsPopulated := false
	processedFields := make(map[string]bool)

	// // First process all Coding fields
	// for i := 0; i < structType.NumField(); i++ {
	// 	field := structValue.Field(i)
	// 	fieldType := field.Type().String()
	// 	fieldName := structType.Field(i).Name

	// 	if strings.HasSuffix(fieldType, "Coding") || strings.HasSuffix(fieldType, "[]Coding") {
	// 		codingPath := fmt.Sprintf("%s.%s", structPath, strings.ToLower(fieldName))

	// 		// Mark this path as processed
	// 		rp.processedPaths[codingPath] = true

	// 		// Get the rows for this specific coding path
	// 		codingRows, exists := result[codingPath]
	// 		if !exists {
	// 			rp.log.Debug().
	// 				Str("codingPath", codingPath).
	// 				Msg("No data found for coding path")
	// 			continue
	// 		}

	// 		// Find the matching coding row using the parent ID
	// 		var codingRow RowData
	// 		for _, r := range codingRows {
	// 			if r.ParentID == row.ID {
	// 				codingRow = r
	// 				break
	// 			}
	// 		}

	// 		if codingRow.ID == "" {
	// 			rp.log.Debug().
	// 				Str("codingPath", codingPath).
	// 				Str("parentID", row.ID).
	// 				Msg("No matching coding row found")
	// 			continue
	// 		}

	// 		// Mark all coding-related fields as processed
	// 		for key := range codingRow.Data {
	// 			keyLower := strings.ToLower(key)
	// 			if strings.HasSuffix(keyLower, "code") ||
	// 				strings.HasSuffix(keyLower, "display") ||
	// 				strings.HasSuffix(keyLower, "system") {
	// 				processedFields[key] = true
	// 				// Also mark the original field name as processed
	// 				processedFields[fieldName] = true
	// 			}
	// 		}

	// 		if err := rp.setCodingFromRow(codingPath, field, fieldName, codingRow, processedFields); err != nil {
	// 			return nil, err
	// 		}
	// 		fieldsPopulated = true
	// 	}
	// }

	// Then process regular fields that haven't been handled as part of a Coding
	for key, value := range row.Data {
		// Skip if this field was already processed as part of a Coding
		if processedFields[key] {
			rp.log.Debug().
				Str("key", key).
				Msg("Skipping already processed field")
			continue
		}

		for i := 0; i < structType.NumField(); i++ {
			fieldName := structType.Field(i).Name
			// Skip if the field was marked as processed during Coding handling
			// if processedFields[fieldName] {
			// 	continue
			// }

			if strings.EqualFold(fieldName, key) {
				if err := rp.setField(structPath, structPtr, fieldName, value); err != nil {
					rp.log.Error().Err(err).
						Str("structPath", structPath).
						Str("fieldName", fieldName).
						Msg("Failed to set field")
					return nil, err
				}
				fieldsPopulated = true

				// Check field filter
				fieldPath := fmt.Sprintf("%s.%s", structPath, strings.ToLower(fieldName))
				if filterResult, err := rp.checkFilter(fieldPath, structValue.Field(i)); err != nil {
					rp.log.Error().Err(err).
						Str("fieldPath", fieldPath).
						Msg("Failed to check filter")
					return nil, err
				} else if !filterResult.Passed {
					rp.log.Debug().
						Str("fieldPath", fieldPath).
						Msg("Field did not pass filter")
					return filterResult, nil
				}
			}
		}
	}

	// Handle ID field if not already processed
	if idField := structValue.FieldByName("Id"); idField.IsValid() && idField.CanSet() && !processedFields["Id"] {
		if err := rp.setField(structPath, structPtr, "Id", row.ID); err != nil {
			return nil, err
		}
		fieldsPopulated = true
	}

	if !fieldsPopulated {
		return &FilterResult{
			Passed:  true,
			Message: fmt.Sprintf("No fields populated for %s", structPath),
		}, nil
	}

	return &FilterResult{Passed: true}, nil
}

func (rp *ResourceProcessor) setCodingFromRow(structPath string, field reflect.Value, fieldName string, row RowData, processedFields map[string]bool) error {
	rp.log.Debug().
		Str("structPath", structPath).
		Str("fieldName", fieldName).
		Interface("rowData", row.Data).
		Msg("Starting Coding field processing")

	// Extract code, display, and system values from the row data
	var code, display, system string

	// Look for fields in row data
	for key, value := range row.Data {
		keyLower := strings.ToLower(key)
		strValue := fmt.Sprint(value)

		rp.log.Trace().
			Str("key", key).
			Str("keyLower", keyLower).
			Str("value", strValue).
			Msg("Checking row data field")

		switch {
		case strings.HasSuffix(keyLower, "code"):
			code = strValue
			processedFields[key] = true
			rp.log.Debug().
				Str("field", key).
				Str("code", code).
				Msg("Found code field")
		case strings.HasSuffix(keyLower, "display"):
			display = strValue
			processedFields[key] = true
			rp.log.Debug().
				Str("field", key).
				Str("display", display).
				Msg("Found display field")
		case strings.HasSuffix(keyLower, "system"):
			system = strValue
			processedFields[key] = true
			rp.log.Debug().
				Str("field", key).
				Str("system", system).
				Msg("Found system field")
		}
	}

	// Log the collected values before mapping
	rp.log.Debug().
		Str("structPath", structPath).
		Str("fieldName", fieldName).
		Str("originalCode", code).
		Str("originalDisplay", display).
		Str("originalSystem", system).
		Msg("Collected values before concept mapping")

	// If we found a code, try to map it
	if code != "" {
		mappedCode, mappedDisplay, err := "mappedCode", "mappedDisplay", error(nil)
		if err != nil {
			rp.log.Warn().
				Err(err).
				Str("structPath", structPath).
				Str("originalCode", code).
				Msg("Concept mapping failed, using original values")
		} else {
			rp.log.Info().
				Str("originalCode", code).
				Str("mappedCode", mappedCode).
				Str("originalDisplay", display).
				Str("mappedDisplay", mappedDisplay).
				Msg("Successfully mapped concept")

			code = mappedCode
			if mappedDisplay != "" {
				display = mappedDisplay
			}
		}

		// Create the Coding
		coding := fhir.Coding{
			Code:    &code,
			Display: nil,
			System:  nil,
		}

		if display != "" {
			coding.Display = &display
			rp.log.Debug().
				Str("display", display).
				Msg("Setting display in Coding")
		}
		if system != "" {
			coding.System = &system
			rp.log.Debug().
				Str("system", system).
				Msg("Setting system in Coding")
		}

		// Check if the field is a slice
		if field.Kind() == reflect.Slice {
			// Create a new slice with one element
			newSlice := reflect.MakeSlice(field.Type(), 0, 1)
			newSlice = reflect.Append(newSlice, reflect.ValueOf(coding))
			field.Set(newSlice)
		} else {
			// If it's not a slice, it should be a single Coding field
			field.Set(reflect.ValueOf(coding))
		}

		rp.log.Debug().
			Str("code", code).
			Str("display", display).
			Str("system", system).
			Msg("Set Coding field")
	} else {
		rp.log.Debug().
			Str("structPath", structPath).
			Str("fieldName", fieldName).
			Msg("No code found in row data")
	}

	// Log processed fields
	rp.log.Debug().
		Interface("processedFields", processedFields).
		Msg("Fields marked as processed")

	return nil
}

// Helper function to set a string pointer if the value is not empty
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Helper function to check if a field name matches a pattern with a suffix
func fieldMatchesPattern(fieldName string, prefix string, suffix string) bool {
	fieldLower := strings.ToLower(fieldName)
	return strings.HasPrefix(fieldLower, strings.ToLower(prefix)) &&
		strings.HasSuffix(fieldLower, strings.ToLower(suffix))
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
