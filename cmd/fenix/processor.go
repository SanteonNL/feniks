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

// Modify ResourceProcessor to track processed paths// First update the ResourceProcessor struct to include the cache
type ResourceProcessor struct {
	resourceType   string
	searchParams   SearchParameterMap
	log            zerolog.Logger
	processedPaths ProcessedPaths
	result         map[string][]RowData
	valueSetCache  *ValueSetCache // Add this field
}

type FilterResult struct {
	Passed  bool
	Message string
}

// Update the constructor to initialize the cache
func NewResourceProcessor(resourceType string, searchParams SearchParameterMap, log zerolog.Logger, result map[string][]RowData) *ResourceProcessor {
	// Initialize the ValueSet cache
	valueSetCache := NewValueSetCache(
		"./valueset", // Local storage path
		log,
	)

	return &ResourceProcessor{
		resourceType:   resourceType,
		searchParams:   searchParams,
		log:            log,
		processedPaths: make(ProcessedPaths),
		result:         result,
		valueSetCache:  valueSetCache,
	}
}

// Update ProcessResources to pass the result map
func ProcessResources(ds *DataSource, patientID string, searchParams SearchParameterMap, log zerolog.Logger) ([]interface{}, error) {
	results, err := ds.ReadResources(patientID)
	if err != nil {
		return nil, fmt.Errorf("error reading data: %w", err)
	}

	WriteToJSON(results, "results", "output/temp", log)

	log.Info().Msgf("Number of results found: %d", len(results))

	var processedResources []interface{}

	for _, result := range results {
		processor := NewResourceProcessor(ds.resourceType, searchParams, log, result)

		resource, err := CreateResource(ds.resourceType)
		if err != nil {
			return nil, fmt.Errorf("error creating resource: %w", err)
		}

		resourceValue := reflect.ValueOf(resource).Elem()

		filterResult, err := processor.populateResourceStruct(resourceValue, result)
		if err != nil {
			log.Error().Err(err).Msg("Error processing resource")
			continue
		}

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

		// Skip if we've already processed this path
		if rp.processedPaths[fieldPath] {
			rp.log.Debug().
				Str("fieldPath", fieldPath).
				Msg("Skipping already processed nested field")
			continue
		}

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
	processedFields := make(map[string]bool)

	// First process all Coding and CodeableConcept fields
	for i := 0; i < structType.NumField(); i++ {
		field := structValue.Field(i)
		fieldType := field.Type().String()
		fieldName := structType.Field(i).Name
		rp.log.Debug().
			Str("fieldType", fieldType).
			Str("fieldName", fieldName).
			Msg("Checking field type")

		// Handle both direct Coding fields and CodeableConcept fields
		if strings.HasSuffix(fieldType, "Coding") ||
			strings.HasSuffix(fieldType, "[]Coding") ||
			strings.HasSuffix(fieldType, "CodeableConcept") ||
			strings.HasSuffix(fieldType, "[]CodeableConcept") {

			codingPath := fmt.Sprintf("%s.%s", structPath, strings.ToLower(fieldName))
			rp.processedPaths[codingPath] = true

			codingRows, exists := result[codingPath]
			if !exists {
				continue
			}

			// Handle CodeableConcept (single or slice)
			if strings.Contains(fieldType, "CodeableConcept") {
				if err := rp.setCodeableConceptField(field, codingPath, fieldName, row.ID, codingRows, processedFields); err != nil {
					// Check if this field has a filter
					if _, hasFilter := rp.searchParams[codingPath]; hasFilter {
						return &FilterResult{
							Passed:  false,
							Message: fmt.Sprintf("Filter failed for %s: %v", codingPath, err),
						}, nil
					}
					// If no filter, continue processing other fields
					rp.log.Debug().Err(err).Str("path", codingPath).Msg("Error processing CodeableConcept field")
					continue
				}
			} else {
				// Handle regular Coding fields
				for _, codingRow := range codingRows {
					if codingRow.ParentID == row.ID {
						if err := rp.setCodingFromRow(codingPath, field, fieldName, codingRow, processedFields); err != nil {
							return nil, err
						}
					}
				}

				// After setting all codings, check filter at the parent level
				if filterResult, err := rp.checkFilter(structPath, structValue); err != nil {
					return nil, err
				} else if !filterResult.Passed {
					rp.log.Debug().
						Str("fieldPath", structPath).
						Msg("Field did not pass filter")
					return filterResult, nil
				}
			}
		}
	}

	// Then process regular fields
	for key, value := range row.Data {
		if processedFields[key] {
			continue
		}

		for i := 0; i < structType.NumField(); i++ {
			fieldName := structType.Field(i).Name
			if processedFields[fieldName] {
				continue
			}

			if strings.EqualFold(fieldName, key) {
				if err := rp.setField(structPath, structPtr, fieldName, value); err != nil {
					return nil, fmt.Errorf("failed to set field %s: %w", fieldName, err)
				}

				// Check field filter
				fieldPath := fmt.Sprintf("%s.%s", structPath, strings.ToLower(fieldName))
				if filterResult, err := rp.checkFilter(fieldPath, structValue.Field(i)); err != nil {
					return nil, fmt.Errorf("failed to check filter for field %s: %w", fieldName, err)
				} else if !filterResult.Passed {
					rp.log.Debug().
						Str("fieldPath", fieldPath).
						Msg("Field did not pass filter")
					return filterResult, nil
				}

				processedFields[fieldName] = true
				break
			}
		}
	}

	// Handle ID field if not already processed
	if idField := structValue.FieldByName("Id"); idField.IsValid() && idField.CanSet() && !processedFields["Id"] {
		if err := rp.setField(structPath, structPtr, "Id", row.ID); err != nil {
			return nil, fmt.Errorf("failed to set Id field: %w", err)
		}
	}

	return &FilterResult{Passed: true}, nil
}

// Modified setCodeableConceptField to better handle row relationships
// Modified setCodeableConceptField to handle filtering
func (rp *ResourceProcessor) setCodeableConceptField(field reflect.Value, path string, fieldName string, parentID string, rows []RowData, processedFields map[string]bool) error {
	rp.log.Debug().
		Str("path", path).
		Str("fieldName", fieldName).
		Str("parentID", parentID).
		Int("rowCount", len(rows)).
		Msg("Setting CodeableConcept field")

	isSlice := field.Kind() == reflect.Slice

	if isSlice {
		// Handle slice of CodeableConcepts
		if field.IsNil() {
			field.Set(reflect.MakeSlice(field.Type(), 0, len(rows)))
		}

		// Group rows by their parent CodeableConcept
		conceptRows := make(map[string][]RowData)
		for _, row := range rows {
			if row.ParentID == parentID {
				conceptRows[row.ID] = append(conceptRows[row.ID], row)
			}
		}

		// Track if any element passes the filter
		anyElementPassed := false
		hasFilter := false
		if _, exists := rp.searchParams[path]; exists {
			hasFilter = true
		}

		// Create a CodeableConcept for each group
		for conceptID, conceptGroup := range conceptRows {
			rp.log.Debug().
				Str("conceptID", conceptID).
				Int("groupSize", len(conceptGroup)).
				Msg("Processing CodeableConcept group")

			newConcept := reflect.New(field.Type().Elem()).Elem()
			if err := rp.populateCodeableConcept(newConcept, path, conceptGroup[0], processedFields); err != nil {
				return err
			}

			// Check filter for this concept if there is one
			if hasFilter {
				filterResult, err := rp.checkFilter(path, newConcept)
				if err != nil {
					return err
				}
				if filterResult.Passed {
					anyElementPassed = true
				}
			}

			// Always add to slice, we'll check filter result later
			field.Set(reflect.Append(field, newConcept))
		}

		// If we have a filter and nothing passed, return error
		if hasFilter && !anyElementPassed {
			rp.log.Debug().
				Str("path", path).
				Msg("No CodeableConcepts passed filter")
			return fmt.Errorf("no matching CodeableConcepts found")
		}
	} else {
		// Handle single CodeableConcept
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			field = field.Elem()
		}

		// Find the main CodeableConcept row
		var conceptRow RowData
		for _, row := range rows {
			if row.ParentID == parentID {
				conceptRow = row
				break
			}
		}

		if conceptRow.ID != "" {
			if err := rp.populateCodeableConcept(field, path, conceptRow, processedFields); err != nil {
				return err
			}

			// Check filter after population
			if _, exists := rp.searchParams[path]; exists {
				filterResult, err := rp.checkFilter(path, field)
				if err != nil {
					return err
				}
				if !filterResult.Passed {
					rp.log.Debug().
						Str("path", path).
						Msg("CodeableConcept did not pass filter")
					return fmt.Errorf("CodeableConcept did not match filter criteria")
				}
			}
		}
	}

	return nil
}

// Modified populateCodeableConcept to better handle Coding population
// Modified populateCodeableConcept to correctly find coding rows
func (rp *ResourceProcessor) populateCodeableConcept(conceptValue reflect.Value, path string, row RowData, processedFields map[string]bool) error {
	rp.log.Debug().
		Str("path", path).
		Str("rowID", row.ID).
		Interface("rowData", row.Data).
		Msg("Populating CodeableConcept")

	// Get the Coding field
	codingField := conceptValue.FieldByName("Coding")
	if !codingField.IsValid() {
		return fmt.Errorf("invalid Coding field in CodeableConcept")
	}

	// Initialize Coding slice
	if codingField.Kind() == reflect.Slice && codingField.IsNil() {
		codingField.Set(reflect.MakeSlice(codingField.Type(), 0, 1))
	}

	// Look up coding rows using the correct path
	codingPath := fmt.Sprintf("%s.coding", path)
	codingRows, exists := rp.result[codingPath]

	rp.log.Debug().
		Str("codingPath", codingPath).
		Bool("exists", exists).
		Int("rowCount", len(codingRows)).
		Msg("Looking up coding rows")

	if exists {
		// Process all coding rows that belong to this CodeableConcept
		for _, codingRow := range codingRows {
			if codingRow.ParentID == row.ID {
				rp.log.Debug().
					Str("codingRowID", codingRow.ID).
					Str("parentID", row.ID).
					Interface("codingData", codingRow.Data).
					Msg("Processing coding row")

				if err := rp.setCodingFromRow(codingPath, codingField, "Coding", codingRow, processedFields); err != nil {
					return fmt.Errorf("failed to set coding: %w", err)
				}
			}
		}
	}

	// Process text field if present
	if textValue, exists := row.Data["text"]; exists {
		textField := conceptValue.FieldByName("Text")
		if textField.IsValid() && textField.CanSet() && textField.Kind() == reflect.Ptr {
			if textField.IsNil() {
				textField.Set(reflect.New(textField.Type().Elem()))
			}
			textField.Elem().SetString(fmt.Sprint(textValue))
			rp.log.Debug().
				Str("text", fmt.Sprint(textValue)).
				Msg("Set text field")
		}
	}

	return nil
}

// Modified setCodingFromRow to add more debug logging
func (rp *ResourceProcessor) setCodingFromRow(structPath string, field reflect.Value, fieldName string, row RowData, processedFields map[string]bool) error {
	rp.log.Debug().
		Str("structPath", structPath).
		Str("fieldName", fieldName).
		Interface("rowData", row.Data).
		Msg("Starting Coding field processing")

	var code, display, system string

	// Extract coding fields
	for key, value := range row.Data {
		keyLower := strings.ToLower(key)
		strValue := fmt.Sprint(value)

		switch {
		case strings.HasSuffix(keyLower, "code"):
			code = strValue
			processedFields[key] = true
			rp.log.Debug().Str("code", code).Msg("Found code")
		case strings.HasSuffix(keyLower, "display"):
			display = strValue
			processedFields[key] = true
			rp.log.Debug().Str("display", display).Msg("Found display")
		case strings.HasSuffix(keyLower, "system"):
			system = strValue
			processedFields[key] = true
			rp.log.Debug().Str("system", system).Msg("Found system")
		}
	}

	if code == "" && system == "" {
		rp.log.Debug().Msg("No code or system found in row data")
		return nil
	}

	// Create the Coding
	coding := fhir.Coding{
		Code:    stringPtr(code),
		Display: stringPtr("mapped display"),
		System:  stringPtr(system),
	}

	// Handle slice vs single coding field
	if field.Kind() == reflect.Slice {
		var newSlice reflect.Value
		if field.IsNil() {
			newSlice = reflect.MakeSlice(field.Type(), 0, 1)
		} else {
			newSlice = field
		}

		// Check for duplicates
		exists := false
		for i := 0; i < newSlice.Len(); i++ {
			existing := newSlice.Index(i).Interface().(fhir.Coding)
			if (existing.Code != nil && coding.Code != nil && *existing.Code == *coding.Code) &&
				(existing.System != nil && coding.System != nil && *existing.System == *coding.System) {
				exists = true
				break
			}
		}

		if !exists {
			newSlice = reflect.Append(newSlice, reflect.ValueOf(coding))
			field.Set(newSlice)
			rp.log.Debug().
				Str("code", code).
				Str("system", system).
				Msg("Added new coding to slice")
		}
	} else {
		field.Set(reflect.ValueOf(coding))
		rp.log.Debug().
			Str("code", code).
			Str("system", system).
			Msg("Set single coding field")
	}

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
