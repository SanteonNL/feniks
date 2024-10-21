package main

import (
	"encoding/json"
	"fmt"
	"unicode"

	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/SanteonNL/fenix/util"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

var FHIRResourceMap = map[string]func() interface{}{
	"Patient":     func() interface{} { return &fhir.Patient{} },
	"Observation": func() interface{} { return &fhir.Observation{} },
}

func main() {
	startTime := time.Now()
	log := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) { w.Out = os.Stdout })).With().Timestamp().Caller().Logger()
	log.Debug().Msg("Starting fenix")

	db, err := sqlx.Connect("postgres", "postgres://postgres:mysecretpassword@localhost:5432/tsl_employee?sslmode=disable")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to the database")
	}
	defer db.Close()

	// Set up data source
	//queryPath := util.GetAbsolutePath("queries/hix/flat/Observation_hix_metingen_metingen.sql")
	queryPath := util.GetAbsolutePath("queries/hix/flat/patient.sql")
	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read query file")
	}
	query := string(queryBytes)
	dataSource := NewSQLDataSource(db, query, log)

	data, err := dataSource.ReadPerPatient("456")
	if err != nil {
		log.Debug().Err(err).Msg("Failed to read data")
	}
	log.Debug().Interface("dataMap", data).Msg("Flat data before mapping to FHIR")

	if len(data) > 0 {
		firstElement := data[0]
		jsonData, err := json.MarshalIndent(firstElement, "", "  ")
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to marshal data to JSON")
		}

		outputPath := "output/temp/dataMap.json"
		err = os.MkdirAll("output/temp", os.ModePerm)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create output directory")
		}

		err = os.WriteFile(outputPath, jsonData, 0644)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to write JSON data to file")
		}

		log.Debug().Str("outputPath", outputPath).Msg("Successfully wrote data to JSON file")
	} else {
		log.Debug().Msg("No data available to write to JSON file")
	}

	//Set up search parameters
	searchParameterMap := SearchParameterMap{
		"Patient.identifier": SearchParameter{
			Code:  "identifier",
			Type:  "token",
			Value: "https://santeon.nl|123",
		},
	}

	// Initialize concept maps
	// TODO: remove after starting to use the conceptmaps from the config folder
	//initializeGenderConceptMap()

	// Load StructureDefinitions
	err = LoadStructureDefinitions(log)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load StructureDefinitions")
	}

	// Check if FhirPathToValueset is filled correctly
	for fhirPath, valueset := range FhirPathToValueset {
		fmt.Printf("Path: %s, Valueset: %s\n", fhirPath, valueset)
	}

	// TODO: integrate with processing all json datasources
	// Load ConceptMaps
	err = LoadConceptMaps(log)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load ConceptMaps")
	}

	// Check if ValueSetToConceptMap is filled correctly
	for valueset, conceptMap := range ValueSetToConceptMap {
		log.Debug().Msgf("Valueset: %s, Conceptmap ID: %s\n", valueset, *conceptMap.Id)
	}

	// TODO
	// Check if FhirPathToValueset is filled correctly
	/*for fhirPath, valueset := range FhirPathToValueset {
		fmt.Printf("Path: %s, Valueset: %s\n", fhirPath, valueset)
	}*/

	// Example usage: Reading a ConceptMap
	/*conceptMap, err := ReadFHIRResource("conceptmaps/2.16.840.1.113883.2.4.3.11.60.124.24.json", fhir.UnmarshalConceptMap)
	if err != nil {
		fmt.Printf("Error reading ConceptMap: %v\n", err)
	} else {
		fmt.Printf("Successfully read ConceptMap with ID: %s\n", *conceptMap.Id)
	}

	// Use the ConceptMap struct
	fmt.Printf("ConceptMap ID: %s\n", *conceptMap.Id)
	fmt.Printf("ConceptMap SoureUri a.k.a. source valuesetname: %s\n", *conceptMap.SourceUri)
	fmt.Printf("ConceptMap TargetUri a.k.a. target valuesetname: %s\n", *conceptMap.TargetUri)

	// Example: Iterate through groups and display elements
	for _, group := range conceptMap.Group {
		fmt.Printf("Source a.k.a. system_source: %s, Target a.k.a. system_target: %s\n", *group.Source, *group.Target)
		for _, element := range group.Element {
			fmt.Printf("Code: %s, Target: %v\n", *element.Code, *element.Target[0].Code)
		}
	}
	*/
	//Process data
	patientID := "123"
	_, err = ProcessDataSource(dataSource, "Observation", patientID, searchParameterMap, log)
	_, err = ProcessDataSource(dataSource, "Patient", patientID, searchParameterMap, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to process data source")
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	log.Debug().Msgf("Execution time: %s", duration)
}

func ProcessDataSource(ds DataSource, resourceType string, patientID string, searchParameterMap SearchParameterMap, log zerolog.Logger) ([]interface{}, error) {
	// Get the factory function for the specified resource type
	factory, ok := FHIRResourceMap[resourceType]
	if !ok {
		return nil, fmt.Errorf("unsupported FHIR resource type: %s", resourceType)
	}

	// Read data for the specified patient
	resourceResults, err := ds.ReadPerPatient(patientID)
	if err != nil {
		return nil, fmt.Errorf("error reading data: %w", err)
	}

	log.Debug().Int("resourceCount", len(resourceResults)).Str("resourceType", resourceType).Str("patientID", patientID).Msg("Retrieved resource data")

	var resources []interface{}

	for _, resourceResult := range resourceResults {
		// Create a new instance of the resource
		resource := factory()
		resourceValue := reflect.ValueOf(resource).Elem()

		// Populate the resource struct
		filterResult, err := populateResourceStruct(resourceType, resourceValue, resourceResult, searchParameterMap, log)
		if err != nil {
			log.Error().Err(err).Str("patientID", patientID).Msg("Error populating resource struct")
			continue
		}

		if !filterResult.Passed {
			log.Info().Str("patientID", patientID).Str("message", filterResult.Message).Msg("Resource filtered out")
			continue
		}

		resources = append(resources, resource)

		// Print the resource using MarshalIndent
		marshalFunc := reflect.ValueOf(resource).MethodByName("MarshalJSON")
		if !marshalFunc.IsValid() {
			log.Error().Str("patientID", patientID).Msg("MarshalJSON method not found")
			continue
		}

		// Output the result
		jsonData, err := json.MarshalIndent(resource, "", "  ")
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to marshal patient to JSON")
		}
		fmt.Println("Patient data:")
		fmt.Println(string(jsonData))

	}

	return resources, nil
}

func populateResourceStruct(resourceType string, field reflect.Value, resourceResult ResourceResult, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {

	filterResult, err := determineType(resourceType, field, "", resourceResult, searchParameterMap, log)
	if err != nil {
		return nil, err
	}
	if !filterResult.Passed {
		return filterResult, nil
	}

	return &FilterResult{Passed: true}, nil
}

func determineType(structPath string, value reflect.Value, parentID string, resourceResult ResourceResult, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Str("StructPath", structPath).Msg("Determining type")
	rows, exists := resourceResult[structPath]
	if !exists {
		log.Debug().Msgf("No data found for: %s", structPath)
		return &FilterResult{Passed: true}, nil
	}

	switch value.Kind() {
	case reflect.Slice:
		log.Debug().Msgf("Type is slice")
		// PopulateSlice handle slices of structs, slices of basic types like string, int, etc. are handled within SetField it seems.
		// However, the patient resource does not contain slices of basic types, so this I am not sure if it works well if for example
		// the resource humanName is filled directly as resource instead of as struct within resource. That contains several slices of strings.
		// TODO: test this with a resource that contains slices of basic types.
		return populateSlice(structPath, value, parentID, rows, resourceResult, searchParameterMap, log)
	case reflect.Struct:
		log.Debug().Str("Structpath", structPath).Msgf("Type is struct")
		return populateStruct(structPath, value, parentID, rows, resourceResult, searchParameterMap, log)
	case reflect.Ptr:
		log.Debug().Str("Structpath", structPath).Msgf("Type is pointer")
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		log.Debug().Str("Structpath", structPath).Msgf("Changed nil pointer to new instance of %s", value.Type().Elem())
		return determineType(structPath, value.Elem(), parentID, resourceResult, searchParameterMap, log)
	default:
		log.Debug().Str("StructPath", structPath).Msgf("Type is basic type")
		return populateBasicType(structPath, value, parentID, rows, structPath, searchParameterMap, log)
	}
}

func populateSlice(structPath string, value reflect.Value, parentID string, rows []RowData, resourceResult ResourceResult, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Str("structPath", structPath).Msg("Populating slice")
	anyElementPassed := false
	for _, row := range rows {
		log.Debug().Str("structPath", structPath).Msgf("Row: %+v and parentID %s", row, parentID)
		if row.ParentID == parentID || row.ParentID == "" {
			log.Debug().Str("structPath", structPath).Msg("Populating slice element")
			valueElement := reflect.New(value.Type().Elem()).Elem()
			if err := populateStructAndNestedFields(structPath, valueElement, row, resourceResult, searchParameterMap, log); err != nil {
				return nil, err
			}

			filterResult, err := ApplyFilter(structPath, valueElement, searchParameterMap, log)
			if err != nil {
				return nil, err
			}

			if filterResult.Passed {
				anyElementPassed = true
				log.Debug().
					Str("StructPath", structPath).
					Msg("Slice element passed filter, continuing slice population")
			}

			// Always add the element to the slice, regardless of filter result
			value.Set(reflect.Append(value, valueElement))
		} else {
			log.Debug().Str("structPath", structPath).Msg("Skipping slice")
		}
	}

	if anyElementPassed {
		return &FilterResult{Passed: true}, nil
	}

	return &FilterResult{Passed: false, Message: fmt.Sprintf("No elements in slice at structpath %s passed filter", structPath)}, nil
}

func populateStruct(structPath string, value reflect.Value, parentID string, rows []RowData, resourceResult ResourceResult, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Str("Structpath", structPath).Msg("Populating struct")
	for _, row := range rows {
		if row.ParentID == parentID || parentID == "" {
			if err := populateStructAndNestedFields(structPath, value, row, resourceResult, searchParameterMap, log); err != nil {
				return nil, err
			}

			filterResult, err := ApplyFilter(structPath, value, searchParameterMap, log)
			log.Debug().Msg("Hello Applying filter")
			if err != nil {
				return nil, err
			}
			if !filterResult.Passed {
				return filterResult, nil
			}

			break // We only need one matching row for struct fields
		}
	}
	return &FilterResult{Passed: true}, nil
}

// TODO nestedelements -> nested felds
func populateStructAndNestedFields(structPath string, valueElement reflect.Value, row RowData, resourceResult ResourceResult, searchParameterMap SearchParameterMap, log zerolog.Logger) error {
	log.Debug().Str("structPath", structPath).Msg("Populating struct and nested fields")
	if err := populateStructFields(structPath, valueElement.Addr().Interface(), row, searchParameterMap, log); err != nil {
		return err
	}

	currentID := row.ID
	return populateNestedFields(structPath, valueElement, resourceResult, currentID, searchParameterMap, log)
}

func populateNestedFields(parentName string, parentValue reflect.Value, resourceResult ResourceResult, parentID string, searchParameterMap SearchParameterMap, log zerolog.Logger) error {
	log.Debug().Msgf("Populating nested fields for %s", parentName)
	for i := 0; i < parentValue.NumField(); i++ {
		childValue := parentValue.Field(i)
		childFieldName := parentValue.Type().Field(i).Name
		childName := fmt.Sprintf("%s.%s", parentName, strings.ToLower(childFieldName))

		if hasDataForPath(resourceResult, childName) {
			filterResult, err := determineType(childName, childValue, parentID, resourceResult, searchParameterMap, log)
			if err != nil {
				return err
			}
			if !filterResult.Passed {
				return fmt.Errorf(filterResult.Message)
			}
		}
	}
	return nil
}

// TODO add data that use this function.../ remove if unnecessary
// Not yet renamed as in other functions and contains both name and fieldName which is the same (But neede for SetField input)
// TODO ApplyFilter is used in this function but as setting basic types is not using this function they are not filtered also it seems...
// Or maybe not because filtereing is only for certain types?
func populateBasicType(name string, field reflect.Value, parentID string, rows []RowData, fieldName string, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Msgf("Populating basic type field: %s", field)
	for _, row := range rows {
		if row.ParentID == parentID || parentID == "" {
			for key, value := range row.Data {
				if strings.EqualFold(key, fieldName) {
					if err := SetField(fieldName, field.Addr().Interface(), name, value, log); err != nil {
						return nil, err
					}
					return ApplyFilter(fieldName, field, searchParameterMap, log)
				}
			}
		}
	}
	return &FilterResult{Passed: true}, nil
}

// I think this is now used instead of populateBasicType
func populateStructFields(structPath string, structPointer interface{}, row RowData, searchParameterMap SearchParameterMap, log zerolog.Logger) error {
	log.Debug().Str("Structpath", structPath).Msg("Populating structfields")
	structValue := reflect.ValueOf(structPointer).Elem() // This is yet an empty struct
	structType := structValue.Type()
	log.Debug().Msgf("Struct type: %s", structType.Name())

	for key, value := range row.Data {
		for i := 0; i < structType.NumField(); i++ {
			fieldName := structType.Field(i).Name
			fieldNameLower := strings.ToLower(fieldName)

			if fieldNameLower == strings.ToLower(key) {
				if err := SetField(structPath, structPointer, fieldName, value, log); err != nil {
					return err
				}
				fhirPath := structPath + "." + strings.ToLower(fieldName)
				// TODO: I think this might for a code/system not be the right place for the filter?
				// Mogelijk aalleen op bovenste niveau gaat het mis?
				if _, err := ApplyFilter(fhirPath, reflect.ValueOf(value), searchParameterMap, log); err != nil {
					log.Debug().Msgf("Not hello to apply filter: %v", err)
					return err
				}
			}
		}
	}

	// Do conceptmapping for Coding and Quantity

	// CodeableConcept does not need to be checked, as it is a struct with a Coding field
	if structType.Name() == "Coding" || structType.Name() == "Quantity" {
		fhirPath := getValueSetBindingPath(structPath, structType.Name())
		log.Debug().Msgf("fhirPath to determine Valuest: %s", fhirPath)
		_, exists := FhirPathToValueset[fhirPath] // this is the same as in setField
		if exists {
			log.Debug().Msgf("fhirPath %s is in FhirPathToValueset", fhirPath)

			// Collect the conceptmaps for the fhirPath
			conceptMap, err := getConceptMapForFhirPath(fhirPath, log)
			if err != nil {
				log.Debug().Msgf("failed to get conceptmap for fhirPath: %v", err)
			} else {
				log.Debug().Msgf("ConceptMap for fhirPath %s: %v", fhirPath, *conceptMap.Id)

				for i := 0; i < structType.NumField(); i++ {
					fieldName := structType.Field(i).Name
					fieldNameLower := strings.ToLower(fieldName)

					if fieldNameLower == "system" || fieldNameLower == "code" || fieldNameLower == "display" {
						log.Debug().Msgf("This field might need conceptmapping: %s", fieldNameLower)
					}
				}

				// Set the code field if it exists in the struct
				codeField := structValue.FieldByName("Code")
				codeFieldValue := getStringValue(codeField.Elem())
				log.Debug().Msgf("Curent code fieldvalue: %s", codeFieldValue)

				targetCode, targetDisplay, err := TranslateCode(conceptMap, &codeFieldValue, log)
				if err != nil {
					return fmt.Errorf("failed to map concept code: %v", err)
				}
				log.Debug().Msgf("Mapped concept code: %v", *targetCode)

				if codeField.IsValid() && codeField.CanSet() {
					if err := SetField(structPath, structPointer, "Code", *targetCode, log); err != nil {
						return err
					}
				}
				log.Debug().Msgf("targetDisplay could be used for display: %s", *targetDisplay)

				// Step 3: If the target has a mapped code, use that as the inputValue
				/*if *targetCode != "" {
					stringInputValue = *targetCode // Use the mapped code
					log.Debug().Msgf("Using mapped code: %s", stringInputValue)
					log.Debug().Msgf("Using mapped display: %s", *targetDisplay) // not yet changed
				}

				inputValue = stringInputValue
				log.Debug().Msgf("inputValue after concept mapping: %v", inputValue)*/
			}
		} else {
			log.Debug().Msgf("fhirPath %s is not in FhirPathToValueset", fhirPath)
		}
	}

	// Set the ID field if it exists in the struct
	idField := structValue.FieldByName("Id")
	if idField.IsValid() && idField.CanSet() {
		if err := SetField(structPath, structPointer, "Id", row.ID, log); err != nil {
			return err
		}
	}
	return nil
}

// This function is used to set the value of a field in a struct
// It actually also does a lot of validation and conversion of the value to the correct type
func SetField(structPath string, structPointer interface{}, structFieldName string, inputValue interface{}, log zerolog.Logger) error {
	log.Debug().Msgf("Setting structPath %s field %s to %v", structPath, structFieldName, inputValue)

	// Do some checks to ensure that the structField can be set
	structValue := reflect.ValueOf(structPointer)
	if structValue.Kind() != reflect.Ptr || structValue.IsNil() {
		return fmt.Errorf("structPointer must be a non-nil pointer to a struct")
	}

	structValueElement := structValue.Elem()
	if structValueElement.Kind() != reflect.Struct {
		return fmt.Errorf("structPointer must point to a struct type")
	}

	structField := structValueElement.FieldByName(structFieldName)
	if !structField.IsValid() {
		return fmt.Errorf("no such field: %s in struct", structFieldName)
	}

	if !structField.CanSet() {
		return fmt.Errorf("cannot set field %s", structFieldName)
	}

	// Set the structField to it's zero value if value is nil
	if inputValue == nil {
		structField.Set(reflect.Zero(structField.Type()))
		return nil
	}

	// Do conceptmapping for codes
	structFieldType := structField.Type()
	log.Debug().Msgf("Struct type: %s", structFieldType)
	if typeHasCodeMethod(structFieldType) { // Suggesting it is a code type
		fmt.Println("The type has a Code() method, likely indicating a 'code' type.")
		fhirPath := structPath + "." + strings.ToLower(structFieldName)
		fhirPath = extractAndCapitalizeLastTwoParts(fhirPath)
		log.Debug().Msgf("fhirPath: %s", fhirPath)

		// Check if fhirPath is in FhirPathToValueset
		_, exists := FhirPathToValueset[fhirPath]
		if exists {
			log.Debug().Msgf("fhirPath %s is in FhirPathToValueset", fhirPath)

			// Collect the conceptmaps for the fhirPath
			conceptMap, err := getConceptMapForFhirPath(fhirPath, log)
			if err != nil {
				log.Debug().Msgf("failed to get conceptmap for fhirPath: %v", err)
			} else {
				log.Debug().Msgf("ConceptMap for fhirPath %s: %v", fhirPath, *conceptMap.Id)

				// Step 1: Convert inputValue to a string for concept mapping
				inputValueType := reflect.TypeOf(inputValue)
				log.Debug().Msgf("inputValueType: %s", inputValueType)
				stringInputValue := getStringValue(reflect.ValueOf(inputValue))
				log.Debug().Msgf("Converted inputValue to string: %s", stringInputValue)

				targetCode, targetDisplay, err := TranslateCode(conceptMap, &stringInputValue, log)
				if err != nil {
					return fmt.Errorf("failed to map concept code: %v", err)
				}
				log.Debug().Msgf("Mapped concept code: %v", *targetCode)

				// Step 3: If the target has a mapped code, use that as the inputValue
				if *targetCode != "" {
					stringInputValue = *targetCode // Use the mapped code
					log.Debug().Msgf("Using mapped code: %s", stringInputValue)
					log.Debug().Msgf("Using mapped display: %s", *targetDisplay) // not yet changed
				}

				inputValue = stringInputValue
				log.Debug().Msgf("inputValue after concept mapping: %v", inputValue)
			}
		} else {
			log.Debug().Msgf("fhirPath %s is not in FhirPathToValueset", fhirPath)
		}
	}

	// Try UnmarshalJSON for the field and its address
	for _, field := range []reflect.Value{structField, structField.Addr()} {
		if field.CanInterface() && field.Type().Implements(reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()) {
			// Prevent panic when field is a nil pointer
			if field.Kind() == reflect.Ptr {
				if field.IsNil() {
					field.Set(reflect.New(field.Type().Elem()))
				}
			}
			byteValue, err := getByteValue(inputValue)
			if err != nil {
				return fmt.Errorf("failed to convert input to []byte: %v", err)
			}

			method := field.MethodByName("UnmarshalJSON")
			results := method.Call([]reflect.Value{reflect.ValueOf(byteValue)})
			if len(results) > 0 && !results[0].IsNil() {
				return results[0].Interface().(error)
			}
			return nil
		}
	}

	structFieldInputValue := reflect.ValueOf(inputValue)

	// Handle conversion from []uint8 to []string if needed
	// TODO scheck if it should be inputValue or structFieldInputValue
	if structField.Type() == reflect.TypeOf([]string{}) && structFieldInputValue.Type() == reflect.TypeOf([]uint8{}) {
		log.Debug().Msgf("Converting []uint8 to []string for field %s", structFieldName)
		var strSlice []string
		if err := json.Unmarshal(inputValue.([]uint8), &strSlice); err != nil {
			return fmt.Errorf("failed to unmarshal []uint8 to []string: %v", err)
		}
		structField.Set(reflect.ValueOf(strSlice))
		return nil
	}

	if structField.Kind() == reflect.Ptr && (structField.Type().Elem().Kind() == reflect.String || structField.Type().Elem().Kind() == reflect.Bool) {
		var convertedValue reflect.Value

		switch structField.Type().Elem().Kind() {
		case reflect.String:
			var stringValue string
			switch typedInputValue := inputValue.(type) {
			case string:
				stringValue = typedInputValue
			case int, int8, int16, int32, int64:
				stringValue = fmt.Sprintf("%d", typedInputValue)
			case uint, uint8, uint16, uint32, uint64:
				stringValue = fmt.Sprintf("%d", typedInputValue)
			case float32, float64:
				stringValue = fmt.Sprintf("%f", typedInputValue)
			case bool:
				stringValue = strconv.FormatBool(typedInputValue)
			case time.Time:
				stringValue = typedInputValue.Format(time.RFC3339)
			default:
				return fmt.Errorf("cannot convert %T to *string", inputValue)
			}
			convertedValue = reflect.ValueOf(&stringValue)
		case reflect.Bool:
			var boolValue bool
			switch typedInputValue := inputValue.(type) {
			case bool:
				boolValue = typedInputValue
			case string:
				var err error
				boolValue, err = strconv.ParseBool(typedInputValue)
				if err != nil {
					return fmt.Errorf("cannot convert string to *bool: %s", typedInputValue)
				}
			default:
				return fmt.Errorf("cannot convert %T to *bool", inputValue)
			}
			convertedValue = reflect.ValueOf(&boolValue)
		}

		structField.Set(convertedValue)
	} else {
		if structField.Type() != structFieldInputValue.Type() {
			return fmt.Errorf("provided value type didn't match struct field type %s for field %s and %s ", structField.Type(), structFieldName, structFieldInputValue.Type())
		}

		structField.Set(structFieldInputValue)
		log.Debug().Msgf("Set field %s to %v", structFieldName, &structFieldInputValue)
	}

	return nil
}

func getByteValue(v interface{}) ([]byte, error) {
	switch value := v.(type) {
	case string:
		return []byte(value), nil
	case []byte:
		return value, nil
	default:
		// If it's not a string or []byte, try to marshal it to JSON
		return json.Marshal(v)
	}
}

func hasDataForPath(resultMap map[string][]RowData, path string) bool {
	if _, exists := resultMap[path]; exists {
		return true
	}

	return false
}

// Helper function to capitalize the first letter and make the rest lowercase
func capitalizeFirstLetter(s string) string {
	if len(s) == 0 {
		return s
	}
	// Convert first rune to uppercase and the rest to lowercase
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}
