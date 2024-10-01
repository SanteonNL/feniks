package main

import (
	"encoding/json"
	"fmt"

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

// ConceptMapperMap is a nested map structure for efficient lookups
// The structure is: fhirPath -> sourceSystem -> sourceCode -> target
type ConceptMapperMap map[string]map[string]map[string]target

// TargetCode represents the mapped code in the target system
type target struct {
	system  string
	code    string
	display string
}

var globalConceptMaps ConceptMapperMap

func initializeGenderConceptMap() {
	globalConceptMaps = ConceptMapperMap{
		"Patient.gender": {
			"http://hl7.org/fhir/administrative-gender": {
				"male": target{
					system:  "http://snomed.info/sct",
					code:    "248153007",
					display: "Male",
				},
				"female": target{
					system:  "http://snomed.info/sct",
					code:    "248152002",
					display: "Female",
				},
				"other": target{
					system:  "http://snomed.info/sct",
					code:    "394743007",
					display: "Other",
				},
				"unknown": target{
					system:  "http://snomed.info/sct",
					code:    "unknown",
					display: "Unknown",
				},
			},
			"": { // For system-agnostic mappings
				"M": target{
					system:  "http://hl7.org/fhir/administrative-gender",
					code:    "male",
					display: "Male",
				},
				"F": target{
					system:  "http://hl7.org/fhir/administrative-gender",
					code:    "female",
					display: "Female",
				},
				"O": target{
					system:  "http://hl7.org/fhir/administrative-gender",
					code:    "other",
					display: "Other",
				},
				"U": target{
					system:  "http://hl7.org/fhir/administrative-gender",
					code:    "unknown",
					display: "Unknown",
				},
			},
		},
	}
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
	queryPath := util.GetAbsolutePath("queries/hix/flat/patient.sql")
	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read query file")
	}
	query := string(queryBytes)
	dataSource := NewSQLDataSource(db, query, log)

	// Set up search parameters
	searchParameterMap := SearchParameterMap{
		"Patient.identifier": SearchParameter{
			Code:  "identifier",
			Type:  "token",
			Value: "https://santeon.nl|123",
		},
	}

	// Initialize concept maps
	initializeGenderConceptMap()

	// Process data
	patient, filterMessage, err := ProcessDataSource(dataSource, searchParameterMap, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to process data source")
	}

	if filterMessage != "" {
		log.Info().Msg(filterMessage)
		fmt.Println("No data returned due to filtering")
	} else {
		// Output the result
		jsonData, err := json.MarshalIndent(patient, "", "  ")
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to marshal patient to JSON")
		}
		fmt.Println("Patient data:")
		fmt.Println(string(jsonData))
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	log.Debug().Msgf("Execution time: %s", duration)
}

func ProcessDataSource(ds DataSource, searchParameterMap SearchParameterMap, log zerolog.Logger) (*fhir.Patient, string, error) {
	data, err := ds.Read()
	if err != nil {
		return nil, "", err
	}
	log.Debug().Interface("dataMap", data).Msg("Flat data before mapping to FHIR")

	patient := &fhir.Patient{}
	patientStructValue := reflect.ValueOf(patient).Elem()

	filterResult, err := populateResourceStruct("", patientStructValue, "", data, searchParameterMap, log)
	if err != nil {
		return nil, "", err
	}

	if !filterResult.Passed {
		return &fhir.Patient{}, filterResult.Message, nil
	}

	return patient, "", nil
}

func populateResourceStruct(structPath string, value reflect.Value, parentID string, resultMap map[string][]map[string]interface{}, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	if structPath == "" {
		structPath = value.Type().Name()
		log.Debug().Str("Name", structPath).Msg("Populating resource struct")
	}

	filterResult, err := determineType(structPath, value, parentID, resultMap, searchParameterMap, log)
	if err != nil {
		return nil, err
	}
	if !filterResult.Passed {
		return filterResult, nil
	}

	return &FilterResult{Passed: true}, nil
}

func determineType(structPath string, value reflect.Value, parentID string, resultMap map[string][]map[string]interface{}, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Str("StructPath", structPath).Msg("Determining type")
	rows, exists := resultMap[structPath]
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
		return populateSlice(structPath, value, parentID, rows, resultMap, searchParameterMap, log)
	case reflect.Struct:
		log.Debug().Str("Structpath", structPath).Msgf("Type is struct")
		return populateStruct(structPath, value, parentID, rows, resultMap, searchParameterMap, log)
	case reflect.Ptr:
		log.Debug().Str("Structpath", structPath).Msgf("Type is pointer")
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		log.Debug().Str("Structpath", structPath).Msgf("Changed nil pointer to new instance of %s", value.Type().Elem())
		return determineType(structPath, value.Elem(), parentID, resultMap, searchParameterMap, log)
	default:
		log.Debug().Str("StructPath", structPath).Msgf("Type is basic type")
		return populateBasicType(structPath, value, parentID, rows, structPath, searchParameterMap, log)
	}
}

func populateSlice(structPath string, value reflect.Value, parentID string, rows []map[string]interface{}, resultMap map[string][]map[string]interface{}, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Str("structPath", structPath).Msg("Populating slice")
	anyElementPassed := false
	for _, row := range rows {
		if row["parent_id"] == parentID || parentID == "" {
			valueElement := reflect.New(value.Type().Elem()).Elem()
			if err := populateStructAndNestedFields(structPath, valueElement, row, resultMap, searchParameterMap, log); err != nil {
				return nil, err
			}

			filterResult, err := applyFilter(valueElement, structPath, searchParameterMap, log)
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
		}
	}

	if anyElementPassed {
		return &FilterResult{Passed: true}, nil
	}

	return &FilterResult{Passed: false, Message: fmt.Sprintf("No elements in slice at structpath %s passed filter", structPath)}, nil
}

func populateStruct(structPath string, value reflect.Value, parentID string, rows []map[string]interface{}, resultMap map[string][]map[string]interface{}, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Str("Structpath", structPath).Msg("Populating struct")
	for _, row := range rows {
		if row["parent_id"] == parentID || parentID == "" {
			if err := populateStructAndNestedFields(structPath, value, row, resultMap, searchParameterMap, log); err != nil {
				return nil, err
			}

			filterResult, err := applyFilter(value, structPath, searchParameterMap, log)
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
func populateStructAndNestedFields(structPath string, valueElement reflect.Value, row map[string]interface{}, resultMap map[string][]map[string]interface{}, searchParameterMap SearchParameterMap, log zerolog.Logger) error {
	log.Debug().Str("structPath", structPath).Msg("Populating struct and nested fields")
	if err := populateStructFields(structPath, valueElement.Addr().Interface(), row, log); err != nil {
		return err
	}

	currentID, _ := row["id"].(string)
	return populateNestedFields(structPath, valueElement, resultMap, currentID, searchParameterMap, log)
}

func populateNestedFields(parentName string, parentValue reflect.Value, resultMap map[string][]map[string]interface{}, parentID string, searchParameterMap SearchParameterMap, log zerolog.Logger) error {
	log.Debug().Msgf("Populating nested fields for %s", parentName)
	for i := 0; i < parentValue.NumField(); i++ {
		childValue := parentValue.Field(i)
		childFieldName := parentValue.Type().Field(i).Name
		childName := fmt.Sprintf("%s.%s", parentName, strings.ToLower(childFieldName))

		if hasDataForPath(resultMap, childName) {
			filterResult, err := determineType(childName, childValue, parentID, resultMap, searchParameterMap, log)
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
func populateBasicType(name string, field reflect.Value, parentID string, rows []map[string]interface{}, fieldName string, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Msgf("Populating basic type field: %s", field)
	for _, row := range rows {
		if row["parent_id"] == parentID || parentID == "" {
			for key, value := range row {
				if strings.EqualFold(key, fieldName) {
					if err := SetField(fieldName, field.Addr().Interface(), name, value, log); err != nil {
						return nil, err
					}
					return applyFilter(field, fieldName, searchParameterMap, log)
				}
			}
		}
	}
	return &FilterResult{Passed: true}, nil
}

func populateStructFields(structPath string, structPointer interface{}, row map[string]interface{}, log zerolog.Logger) error {
	log.Debug().Str("Structpath", structPath).Msg("Populating structfields")
	structValue := reflect.ValueOf(structPointer).Elem() // This is yet an empty struct
	structType := structValue.Type()
	log.Debug().Msgf("Struct type: %s", structType.Name())

	for key, value := range row {
		for i := 0; i < structType.NumField(); i++ {
			fieldName := structType.Field(i).Name
			fieldNameLower := strings.ToLower(fieldName)

			if fieldNameLower == strings.ToLower(key) {
				err := SetField(structPath, structPointer, fieldName, value, log)
				if err != nil {
					return err
				}
			}
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

	// Check if the field is a pointer to a type that implements UnmarshalJSON
	if structField.Kind() == reflect.Ptr {
		if structField.IsNil() {
			structField.Set(reflect.New(structField.Type().Elem()))
		}
		unmarshalJSONMethod := structField.MethodByName("UnmarshalJSON")
		if unmarshalJSONMethod.IsValid() {
			//Map value first
			stringInputValue := getStringValue(reflect.ValueOf(inputValue))
			//realFhirPath := fhirPath + "." + strings.ToLower(name)
			// log.Debug().Msgf("RealfhirPath: %s, Field: %s, Value: %v", realFhirPath, name, stringValue)
			target, err := mapConceptCode(stringInputValue, "Patient.gender", log)
			if err != nil {
				return fmt.Errorf("failed to map concept code: %v", err)
			}
			inputValue = target.code
			// Convert the value to JSON
			jsonInputValue, err := json.Marshal(inputValue)
			if err != nil {
				return fmt.Errorf("failed to marshal value to JSON: %v", err)
			}

			// Call UnmarshalJSON
			results := unmarshalJSONMethod.Call([]reflect.Value{reflect.ValueOf(jsonInputValue)})
			if len(results) > 0 && !results[0].IsNil() {
				return results[0].Interface().(error)
			}
			return nil
		}
	}

	structFieldInputValue := reflect.ValueOf(inputValue)

	// Handle conversion from []uint8 to []string if needed
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
func applyFilter(field reflect.Value, fhirPath string, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	searchParameter, ok := searchParameterMap[fhirPath]
	if !ok {
		log.Debug().
			Str("field", fhirPath).
			Msg("No filter found for fhirPath")
		return &FilterResult{Passed: true, Message: fmt.Sprintf("No filter defined for: %s", fhirPath)}, nil
	}

	if field.Kind() == reflect.Slice {
		// For slices, we delegate to populateSlice which now handles the filtering
		return &FilterResult{Passed: true}, nil
	}

	filterResult, err := FilterField(field, searchParameter, fhirPath, log)
	if err != nil {
		return nil, err
	}
	log.Debug().
		Str("field", fhirPath).
		Bool("passed", filterResult.Passed).
		Msg("Apply filter result")
	if !filterResult.Passed {
		return &FilterResult{Passed: false, Message: fmt.Sprintf("Field filtered out: %s", fhirPath)}, nil
	}

	return &FilterResult{Passed: true}, nil
}

func hasDataForPath(resultMap map[string][]map[string]interface{}, path string) bool {
	if _, exists := resultMap[path]; exists {
		return true
	}

	return false
}

func extendFhirPath(parentPath, childName string) string {
	return fmt.Sprintf("%s.%s", parentPath, strings.ToLower(childName))
}

func mapConceptCode(value string, fhirPath string, log zerolog.Logger) (target, error) {
	// Simple implementation without system handling
	log.Debug().Str("fhirPath", fhirPath).Str("sourceCode", value).Msg("Mapping concept code")
	if conceptMap, ok := globalConceptMaps[fhirPath]; ok {
		if systemMap, ok := conceptMap[""]; ok {
			if target, ok := systemMap[value]; ok {
				log.Debug().
					Str("fhirPath", fhirPath).
					Str("sourceCode", value).
					Str("targetCode", target.code).
					Msg("Applied concept mapping")
				return target, nil
			}
		}
	}

	// If no mapping found, return the original value
	return target{code: value}, nil
}
