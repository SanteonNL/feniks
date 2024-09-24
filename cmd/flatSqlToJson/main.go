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
// The structure is: fhirPath -> sourceSystem -> sourceCode -> TargetCode
type ConceptMapperMap map[string]map[string]map[string]TargetCode

// TargetCode represents the mapped code in the target system
type TargetCode struct {
	System  string
	Code    string
	Display string
}

var globalConceptMaps ConceptMapperMap

func initializeGenderConceptMap() {
	globalConceptMaps = ConceptMapperMap{
		"Patient.gender": {
			"http://hl7.org/fhir/administrative-gender": {
				"male": TargetCode{
					System:  "http://snomed.info/sct",
					Code:    "248153007",
					Display: "Male",
				},
				"female": TargetCode{
					System:  "http://snomed.info/sct",
					Code:    "248152002",
					Display: "Female",
				},
				"other": TargetCode{
					System:  "http://snomed.info/sct",
					Code:    "394743007",
					Display: "Other",
				},
				"unknown": TargetCode{
					System:  "http://snomed.info/sct",
					Code:    "unknown",
					Display: "Unknown",
				},
			},
			"": { // For system-agnostic mappings
				"M": TargetCode{
					System:  "http://hl7.org/fhir/administrative-gender",
					Code:    "male",
					Display: "Male",
				},
				"F": TargetCode{
					System:  "http://hl7.org/fhir/administrative-gender",
					Code:    "female",
					Display: "Female",
				},
				"O": TargetCode{
					System:  "http://hl7.org/fhir/administrative-gender",
					Code:    "other",
					Display: "Other",
				},
				"U": TargetCode{
					System:  "http://hl7.org/fhir/administrative-gender",
					Code:    "unknown",
					Display: "Unknown",
				},
			},
		},
	}
}

func main() {
	startTime := time.Now()
	log := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) { w.Out = os.Stdout })).With().Timestamp().Caller().Logger()
	log.Debug().Msg("Starting fenix")

	db, err := sqlx.Connect("postgres", "postgres://postgres:mysecretpassword@localhost:5432/public?sslmode=disable")
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

func populateResourceStruct(name string, value reflect.Value, parentID string, resultMap map[string][]map[string]interface{}, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	if name == "" {
		name = value.Type().Name()
		log.Debug().Str("Name", name).Msg("Populating resource struct")
	}

	filterResult, err := determineType(name, value, parentID, resultMap, searchParameterMap, log)
	if err != nil {
		return nil, err
	}
	if !filterResult.Passed {
		return filterResult, nil
	}

	return &FilterResult{Passed: true}, nil
}

func determineType(name string, value reflect.Value, parentID string, resultMap map[string][]map[string]interface{}, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Str("Name", name).Msg("Determining type")
	rows, exists := resultMap[name]
	if !exists {
		log.Debug().Msgf("No data found for: %s", name)
		return &FilterResult{Passed: true}, nil
	}

	switch value.Kind() {
	case reflect.Slice:
		log.Debug().Msgf("Type is slice")
		// PopulateSlice handle slices of structs, slices of basic types like string, int, etc. are handled within SetField it seems.
		// However, the patient resource does not contain slices of basic types, so this I am not sure if it works well if for example
		// the resource humanName is filled directly as resource instead of as struct within resource. That contains several slices of strings.
		// TODO: test this with a resource that contains slices of basic types.
		return populateSlice(name, value, parentID, rows, resultMap, searchParameterMap, log)
	case reflect.Struct:
		log.Debug().Str("Name", name).Msgf("Type is struct")
		return populateStruct(name, value, parentID, rows, resultMap, searchParameterMap, log)
	case reflect.Ptr:
		log.Debug().Str("Name", name).Msgf("Type is pointer")
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		log.Debug().Str("Name", name).Msgf("Changed nil pointer to new instance of %s", value.Type().Elem())
		return determineType(name, value.Elem(), parentID, resultMap, searchParameterMap, log)
	default:
		log.Debug().Str("Name", name).Msgf("Type is basic type")
		return populateBasicType(name, value, parentID, rows, name, searchParameterMap, log)
	}
}

func populateSlice(name string, value reflect.Value, parentID string, rows []map[string]interface{}, resultMap map[string][]map[string]interface{}, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Str("Name", name).Msg("Populating slice")
	anyElementPassed := false
	for _, row := range rows {
		if row["parent_id"] == parentID || parentID == "" {
			elem := reflect.New(value.Type().Elem()).Elem()
			if err := populateStructAndNestedFields(name, elem, row, resultMap, searchParameterMap, log); err != nil {
				return nil, err
			}

			filterResult, err := applyFilter(elem, name, searchParameterMap, log)
			if err != nil {
				return nil, err
			}

			if filterResult.Passed {
				anyElementPassed = true
				log.Debug().
					Str("Name", name).
					Msg("Slice element passed filter, continuing slice population")
			}

			// Always add the element to the slice, regardless of filter result
			value.Set(reflect.Append(value, elem))
		}
	}

	if anyElementPassed {
		return &FilterResult{Passed: true}, nil
	}

	return &FilterResult{Passed: false, Message: fmt.Sprintf("No elements in slice passed filter: %s", name)}, nil
}

func populateStruct(name string, value reflect.Value, parentID string, rows []map[string]interface{}, resultMap map[string][]map[string]interface{}, searchParameterMap SearchParameterMap, log zerolog.Logger) (*FilterResult, error) {
	log.Debug().Str("Name", name).Msg("Populating struct")
	for _, row := range rows {
		if row["parent_id"] == parentID || parentID == "" {
			if err := populateStructAndNestedFields(name, value, row, resultMap, searchParameterMap, log); err != nil {
				return nil, err
			}

			filterResult, err := applyFilter(value, name, searchParameterMap, log)
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
func populateStructAndNestedFields(name string, elem reflect.Value, row map[string]interface{}, resultMap map[string][]map[string]interface{}, searchParameterMap SearchParameterMap, log zerolog.Logger) error {
	log.Debug().Str("Name", name).Msg("Populating struct and nested fields")
	if err := populateStructFields(name, elem.Addr().Interface(), row, log); err != nil {
		return err
	}

	currentID, _ := row["id"].(string)
	return populateNestedFields(name, elem, resultMap, currentID, searchParameterMap, log)
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
					if err := SetField(field.Addr().Interface(), name, fieldName, value, log); err != nil {
						return nil, err
					}
					return applyFilter(field, fieldName, searchParameterMap, log)
				}
			}
		}
	}
	return &FilterResult{Passed: true}, nil
}

func populateStructFields(name string, obj interface{}, row map[string]interface{}, log zerolog.Logger) error {
	log.Debug().Str("Name", name).Msg("Populating structfields")
	v := reflect.ValueOf(obj).Elem()
	t := v.Type()

	for key, value := range row {
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			fieldNameLower := strings.ToLower(field.Name)

			if fieldNameLower == strings.ToLower(key) {
				err := SetField(obj, name, field.Name, value, log)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func SetField(obj interface{}, name string, fieldName string, value interface{}, log zerolog.Logger) error {
	log.Debug().Msgf("Setting field %s to %v", fieldName, value)
	structValue := reflect.ValueOf(obj)
	if structValue.Kind() != reflect.Ptr || structValue.IsNil() {
		return fmt.Errorf("obj must be a non-nil pointer to a struct")
	}

	structElem := structValue.Elem()
	if structElem.Kind() != reflect.Struct {
		return fmt.Errorf("obj must point to a struct")
	}

	field := structElem.FieldByName(fieldName)
	if !field.IsValid() {
		return fmt.Errorf("no such field: %s in obj", fieldName)
	}

	if !field.CanSet() {
		return fmt.Errorf("cannot set field %s", fieldName)
	}

	if value == nil {
		field.Set(reflect.Zero(field.Type()))
		return nil
	}

	// Check if the field is a pointer to a type that implements UnmarshalJSON
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		unmarshalJSONMethod := field.MethodByName("UnmarshalJSON")
		if unmarshalJSONMethod.IsValid() {
			//Map value first
			stringValue := getStringValue(reflect.ValueOf(value))
			//realFhirPath := fhirPath + "." + strings.ToLower(name)
			// log.Debug().Msgf("RealfhirPath: %s, Field: %s, Value: %v", realFhirPath, name, stringValue)
			TargetCode, err := mapConceptCode(stringValue, "Patient.gender", log)
			if err != nil {
				return fmt.Errorf("failed to map concept code: %v", err)
			}
			value = TargetCode.Code
			// Convert the value to JSON
			jsonValue, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal value to JSON: %v", err)
			}

			// Call UnmarshalJSON
			results := unmarshalJSONMethod.Call([]reflect.Value{reflect.ValueOf(jsonValue)})
			if len(results) > 0 && !results[0].IsNil() {
				return results[0].Interface().(error)
			}
			return nil
		}
	}

	fieldValue := reflect.ValueOf(value)

	// Handle conversion from []uint8 to []string if needed
	if field.Type() == reflect.TypeOf([]string{}) && fieldValue.Type() == reflect.TypeOf([]uint8{}) {
		log.Debug().Msgf("Converting []uint8 to []string for field %s", fieldName)
		var strSlice []string
		if err := json.Unmarshal(value.([]uint8), &strSlice); err != nil {
			return fmt.Errorf("failed to unmarshal []uint8 to []string: %v", err)
		}
		field.Set(reflect.ValueOf(strSlice))
		return nil
	}

	if field.Kind() == reflect.Ptr && (field.Type().Elem().Kind() == reflect.String || field.Type().Elem().Kind() == reflect.Bool) {
		var newValue reflect.Value

		switch field.Type().Elem().Kind() {
		case reflect.String:
			var strValue string
			switch v := value.(type) {
			case string:
				strValue = v
			case int, int8, int16, int32, int64:
				strValue = fmt.Sprintf("%d", v)
			case uint, uint8, uint16, uint32, uint64:
				strValue = fmt.Sprintf("%d", v)
			case float32, float64:
				strValue = fmt.Sprintf("%f", v)
			case bool:
				strValue = strconv.FormatBool(v)
			case time.Time:
				strValue = v.Format(time.RFC3339)
			default:
				return fmt.Errorf("cannot convert %T to *string", value)
			}
			newValue = reflect.ValueOf(&strValue)
		case reflect.Bool:
			var boolValue bool
			switch v := value.(type) {
			case bool:
				boolValue = v
			case string:
				var err error
				boolValue, err = strconv.ParseBool(v)
				if err != nil {
					return fmt.Errorf("cannot convert string to *bool: %s", v)
				}
			default:
				return fmt.Errorf("cannot convert %T to *bool", value)
			}
			newValue = reflect.ValueOf(&boolValue)
		}

		field.Set(newValue)
	} else {
		if field.Type() != fieldValue.Type() {
			return fmt.Errorf("provided value type didn't match obj field type %s for field %s and %s ", field.Type(), fieldName, fieldValue.Type())
		}

		field.Set(fieldValue)
		log.Debug().Msgf("Set field %s to %v", fieldName, &fieldValue)
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

func mapConceptCode(value string, fhirPath string, log zerolog.Logger) (TargetCode, error) {
	// Simple implementation without system handling
	log.Debug().Str("fhirPath", fhirPath).Str("sourceCode", value).Msg("Mapping concept code")
	if conceptMap, ok := globalConceptMaps[fhirPath]; ok {
		if systemMap, ok := conceptMap[""]; ok {
			if targetCode, ok := systemMap[value]; ok {
				log.Debug().
					Str("fhirPath", fhirPath).
					Str("sourceCode", value).
					Str("targetCode", targetCode.Code).
					Msg("Applied concept mapping")
				return targetCode, nil
			}
		}
	}

	// If no mapping found, return the original value
	return TargetCode{Code: value}, nil
}
