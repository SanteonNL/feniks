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
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	startTime := time.Now()

	l := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) { w.Out = os.Stdout })).With().Timestamp().Caller().Logger()

	l.Debug().Msg("Starting flatSqlToJson")

	db, err := sqlx.Connect("postgres", "postgres://postgres:mysecretpassword@localhost:5432/tsl_employee?sslmode=disable")
	if err != nil {
		l.Fatal().Err(err).Msg("Failed to connect to the database")
	}

	queryPath := util.GetAbsolutePath("queries/hix/flat/patient.sql")

	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		l.Fatal().Msgf("Failed to read query file: %s", queryPath)
	}
	query := string(queryBytes)

	SQLDataSource := NewSQLDataSource(db, query)

	mapperPath := util.GetAbsolutePath("config/csv_mappings.json")
	mapper, err := LoadCSVMapperFromConfig(mapperPath)
	if err != nil {
		l.Fatal().Err(err).Msg("Failed to load mapper configuration")
	}

	csvDataSource := NewCSVDataSource("test/data/sim/patient.csv", mapper)

	// Choose which DataSource to use
	var dataSource DataSource
	useCSV := false // Set this to false to use SQL
	if useCSV {
		dataSource = csvDataSource
	} else {
		dataSource = SQLDataSource
	}

	searchFilterGroup := SearchFilterGroup{"Patient.identifier": SearchFilter{Code: "identifier", Type: "token", Value: "https://santeon.nl|123", Expression: "Patient.identifier"}}

	patient := fhir.Patient{}

	err = ExtractAndMapData(dataSource, &patient, searchFilterGroup, l)
	if err != nil {
		l.Fatal().Stack().Err(errors.WithStack(err)).Msg("Failed to populate struct")
		return
	}

	jsonData, err := json.MarshalIndent(patient, "", "  ")
	if err != nil {
		l.Fatal().Err(err).Msg("Failed to marshal patient to JSON")
		return
	}

	fmt.Println("JSON data:")
	fmt.Println(string(jsonData))

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	l.Debug().Msgf("Execution time: %s", duration)
}

func ExtractAndMapData(ds DataSource, s interface{}, sg SearchFilterGroup, logger zerolog.Logger) error {
	data, err := ds.Read()
	if err != nil {
		return err
	}
	logger.Debug().Interface("rawData", data).Msgf("Data before mapping:\n%+v", data)

	v := reflect.ValueOf(s).Elem()
	return populateStruct(v, data, "", "", sg)
}

func populateStruct(field reflect.Value, resultMap map[string][]map[string]interface{}, fieldName string, parentID string, sg SearchFilterGroup) error {
	if fieldName == "" {
		fieldName = field.Type().Name()
	}

	if data, ok := resultMap[fieldName]; ok {
		for _, row := range data {
			if row["parent_id"] == parentID || parentID == "" {
				err := populateStructFromRow(field.Addr().Interface(), row)
				if err != nil {
					return err
				}

				structID := ""
				if idField := field.FieldByName("Id"); idField.IsValid() && idField.Kind() == reflect.Ptr && idField.Type().Elem().Kind() == reflect.String {
					if idField.Elem().IsValid() {
						structID = idField.Elem().String()
					}
				}

				for i := 0; i < field.NumField(); i++ {
					nestedField := field.Field(i)
					nestedFieldName := field.Type().Field(i).Name
					nestedFullFieldName := fieldName + "." + nestedFieldName
					nestedFullFieldName = strings.ToLower(nestedFullFieldName)
					nestedFullFieldName = strings.ToUpper(string(nestedFullFieldName[0])) + nestedFullFieldName[1:]

					if fieldExistsInResultMap(resultMap, nestedFullFieldName) {
						err := populateField(nestedField, resultMap, nestedFullFieldName, structID, sg)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	} else {
		log.Debug().Msgf("No data found for field: %s", fieldName)
	}
	return nil
}

func populateField(field reflect.Value, resultMap map[string][]map[string]interface{}, fieldName string, parentID string, sg SearchFilterGroup) error {
	log.Debug().Msgf("Populating field %s in populateField: %s and parentID", fieldName, parentID)
	switch field.Kind() {
	case reflect.Slice:
		return populateSlice(field, resultMap, fieldName, parentID, sg)
	case reflect.Struct:
		return populateStruct(field, resultMap, fieldName, parentID, sg)
	case reflect.Ptr:
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return populateField(field.Elem(), resultMap, fieldName, parentID, sg)
	default:
		return populateBasicType(field, resultMap, fieldName, parentID)
	}
}

func populateBasicType(field reflect.Value, resultMap map[string][]map[string]interface{}, fullFieldName string, parentID string) error {
	log.Debug().Msgf("Populating basic type fullFieldName: %s", fullFieldName)

	data, ok := resultMap[fullFieldName]
	if !ok {
		return nil
	}

	fieldName := strings.Split(fullFieldName, ".")[len(strings.Split(fullFieldName, "."))-1]
	log.Debug().Msgf("Populating basic type fieldName: %s", fieldName)

	for _, row := range data {
		if row["parent_id"] == parentID || parentID == "" {
			for key, value := range row {
				if strings.EqualFold(key, fieldName) {
					log.Debug().Msgf("Setting field: %s with value: %v", fieldName, value)
					return SetField(field.Addr().Interface(), fieldName, value)
				}
			}
		}
	}
	return nil
}

func populateSlice(field reflect.Value, resultMap map[string][]map[string]interface{}, fieldName string, parentID string, sg SearchFilterGroup) error {
	log.Debug().Msgf("Populating slice field: %s with parentID: %s", fieldName, parentID)
	if data, ok := resultMap[fieldName]; ok {
		for _, row := range data {
			if row["parent_id"] == parentID || parentID == "" {
				newElem := reflect.New(field.Type().Elem()).Elem()
				err := populateStructFromRow(newElem.Addr().Interface(), row)
				if err != nil {
					return err
				}

				newElemID := ""
				if idField := newElem.FieldByName("Id"); idField.IsValid() && idField.Kind() == reflect.Ptr && idField.Type().Elem().Kind() == reflect.String {
					if idField.Elem().IsValid() {
						newElemID = idField.Elem().String()
					}
				}

				for i := 0; i < newElem.NumField(); i++ {
					nestedField := newElem.Field(i)
					nestedFieldName := newElem.Type().Field(i).Name
					nestedFullFieldName := fieldName + "." + nestedFieldName
					nestedFullFieldName = strings.ToLower(nestedFullFieldName)
					nestedFullFieldName = strings.ToUpper(string(nestedFullFieldName[0])) + nestedFullFieldName[1:]

					if fieldExistsInResultMap(resultMap, nestedFullFieldName) {
						err := populateField(nestedField, resultMap, nestedFullFieldName, newElemID, sg)
						if err != nil {
							return err
						}
					}
				}

				field.Set(reflect.Append(field, newElem))
			}
		}
	} else {
		log.Debug().Msgf("No data found for field: %s", fieldName)
	}
	return nil
}

func populateStructFromRow(obj interface{}, row map[string]interface{}) error {
	v := reflect.ValueOf(obj).Elem()
	t := v.Type()

	for key, value := range row {
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			fieldNameLower := strings.ToLower(field.Name)

			if fieldNameLower == strings.ToLower(key) {
				err := SetField(obj, field.Name, value)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func SetField(obj interface{}, name string, value interface{}) error {

	structValue := reflect.ValueOf(obj)
	if structValue.Kind() != reflect.Ptr || structValue.IsNil() {
		return fmt.Errorf("obj must be a non-nil pointer to a struct")
	}

	structElem := structValue.Elem()
	if structElem.Kind() != reflect.Struct {
		return fmt.Errorf("obj must point to a struct")
	}

	field := structElem.FieldByName(name)
	if !field.IsValid() {
		return fmt.Errorf("no such field: %s in obj", name)
	}

	if !field.CanSet() {
		return fmt.Errorf("cannot set field %s", name)
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
			return fmt.Errorf("provided value type didn't match obj field type %s for field %s and %s ", field.Type(), name, fieldValue.Type())
		}

		field.Set(fieldValue)
	}

	return nil
}

func fieldExistsInResultMap(resultMap map[string][]map[string]interface{}, fieldName string) bool {
	fieldName = strings.ToLower(fieldName)
	fieldName = strings.ToUpper(string(fieldName[0])) + fieldName[1:]

	if _, ok := resultMap[fieldName]; ok {
		return true
	}

	for key := range resultMap {
		if strings.HasPrefix(key, fieldName+".") {
			log.Debug().Msgf("Nested field exists in resultMap: %s", key)
			return true
		}
	}
	return false
}
