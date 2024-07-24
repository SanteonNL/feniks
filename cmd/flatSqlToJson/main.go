package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
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

type SearchParameter struct {
	FieldName string
	Value     interface{}
}

type DataSource interface {
	Read() ([]map[string]interface{}, error)
}

type SQLDataSource struct {
	db    *sqlx.DB
	query string
}
type FieldMapping struct {
	FHIRField     string
	FieldName     string
	IDField       string
	ParentIDField string
}

type ColumnMapper struct {
	csvToFHIR        map[string]FieldMapping
	defaultFieldName string
}

type CSVDataSource struct {
	filePath string
	mapper   ColumnMapper
}

func main() {
	startTime := time.Now()

	log := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) { w.Out = os.Stdout })).With().Timestamp().Caller().Logger()

	log.Debug().Msg("Starting flatSqlToJson")

	db, err := sqlx.Connect("postgres", "postgres://postgres:mysecretpassword@localhost:5432/tsl_employee?sslmode=disable")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to the database")
	}

	queryPath := util.GetAbsolutePath("queries/hix/flat/patient_union.sql")

	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		log.Fatal().Msgf("Failed to read query file: %s", queryPath)
	}
	query := string(queryBytes)

	SQLDataSource := NewSQLDataSource(db, query)

	csvToFHIR := map[string]FieldMapping{
		"Identificatienummer": {FHIRField: "id", FieldName: "Patient", IDField: "Identificatienummer", ParentIDField: ""},
		"Geboortedatum":       {FHIRField: "birthDate", FieldName: "Patient", IDField: "Identificatienummer", ParentIDField: ""},
		"Voornaam":            {FHIRField: "text", FieldName: "Patient.Name", IDField: "Identificatienummer", ParentIDField: "Identificatienummer"},
		"Achternaam":          {FHIRField: "family", FieldName: "Patient.Name", IDField: "Identificatienummer", ParentIDField: "Identificatienummer"},
		"GeslachtCode":        {FHIRField: "gender", FieldName: "Patient", IDField: "Identificatienummer", ParentIDField: ""},
	}

	csvMapper := NewColumnMapper(csvToFHIR, "Patient")
	csvDataSource := NewCSVDataSource("test/data/sim/patient.csv", csvMapper)

	// Kies welke DataSource je wilt gebruiken
	var dataSource DataSource
	useCSV := true // Zet dit op false om SQL te gebruiken
	if useCSV {
		dataSource = csvDataSource
	} else {
		dataSource = SQLDataSource
	}

	patient := fhir.Patient{}

	err = ExtractAndMapData(dataSource, &patient)
	if err != nil {
		log.Fatal().Stack().Err(errors.WithStack(err)).Msg("Failed to populate struct")
		return
	}

	jsonData, err := json.MarshalIndent(patient, "", "  ")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to marshal patient to JSON")
		return
	}

	fmt.Println("JSON data:")
	fmt.Println(string(jsonData))

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	log.Debug().Msgf("Execution time: %s", duration)
}

func NewSQLDataSource(db *sqlx.DB, query string) *SQLDataSource {
	return &SQLDataSource{
		db:    db,
		query: query,
	}
}

func (s *SQLDataSource) Read() ([]map[string]interface{}, error) {
	rows, err := s.db.Queryx(s.query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		row := make(map[string]interface{})
		err = rows.MapScan(row)
		if err != nil {
			return nil, err
		}

		// Verwijder NULL waarden
		for key, value := range row {
			if value == nil {
				delete(row, key)
			}
		}

		result = append(result, row)
	}

	return result, nil
}

func NewCSVDataSource(filePath string, mapper ColumnMapper) *CSVDataSource {
	return &CSVDataSource{
		filePath: filePath,
		mapper:   mapper,
	}
}

func (c *CSVDataSource) Read() ([]map[string]interface{}, error) {
	file, err := os.Open(c.filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, header := range headers {
			if mapping, ok := c.mapper.csvToFHIR[header]; ok {
				if record[i] != "" {
					row[mapping.FieldName] = record[i]
				}
			}
		}
		result = append(result, row)
	}

	return result, nil
}

// Helper function to get the index of a column in the headers
func getColumnIndex(headers []string, column string) int {
	for i, header := range headers {
		if header == column {
			return i
		}
	}
	return -1 // Return -1 if the column is not found
}

// Functie om een nieuwe ColumnMapper te maken
func NewColumnMapper(csvToFHIR map[string]FieldMapping, defaultFieldName string) ColumnMapper {
	return ColumnMapper{
		csvToFHIR:        csvToFHIR,
		defaultFieldName: defaultFieldName,
	}
}

func ExtractAndMapData(ds DataSource, s interface{}) error {
	data, err := ds.Read()
	if err != nil {
		return err
	}

	resultMap := make(map[string]map[string][]map[string]interface{})
	for _, row := range data {
		log.Debug().Msgf("Row: %v", row)
		fieldName := row["field_name"].(string)
		parentID := ""
		if pid, ok := row["parent_id"]; ok {
			parentID = fmt.Sprintf("%v", pid)
		}

		if _, ok := resultMap[fieldName]; !ok {
			resultMap[fieldName] = make(map[string][]map[string]interface{})
		}
		resultMap[fieldName][parentID] = append(resultMap[fieldName][parentID], row)
	}

	v := reflect.ValueOf(s).Elem()
	return populateStruct(v, resultMap, "")
}

func populateStruct(v reflect.Value, resultMap map[string]map[string][]map[string]interface{}, parentField string) error {
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("value is not a struct")
	}

	structName := v.Type().Name()
	fullFieldName := parentField

	if parentField == "" {
		fullFieldName = structName
	}

	// Check if there's data for this struct
	if data, ok := resultMap[fullFieldName]; ok {
		for _, rows := range data {
			for _, row := range rows {
				err := populateStructFromRow(v.Addr().Interface(), row)
				if err != nil {
					return err
				}
			}
		}
	}

	structID := ""
	if idField := v.FieldByName("Id"); idField.IsValid() && idField.Kind() == reflect.Ptr && idField.Type().Elem().Kind() == reflect.String {
		if idField.Elem().IsValid() {
			structID = idField.Elem().String()
		}
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldName := v.Type().Field(i).Name
		newFullFieldName := fullFieldName + "." + fieldName

		if fieldExistsInResultMap(resultMap, newFullFieldName) {
			err := populateField(field, resultMap, newFullFieldName, structID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func fieldExistsInResultMap(resultMap map[string]map[string][]map[string]interface{}, fieldName string) bool {
	// Check if the field itself exists
	if _, ok := resultMap[fieldName]; ok {
		// log.Debug().Msgf("Field exists in resultMap: %s", fieldName)
		return true
	}

	// Check if any nested fields exist
	for key := range resultMap {
		if strings.HasPrefix(key, fieldName+".") {
			log.Debug().Msgf("Nested field exists in resultMap: %s", key)
			return true
		}
	}
	return false
}

func populateField(field reflect.Value, resultMap map[string]map[string][]map[string]interface{}, fieldName string, parentID string) error {
	log.Debug().Msgf("Populating field %s in populateField: %s and parentID", fieldName, parentID)
	switch field.Kind() {
	case reflect.Slice:
		return populateSlice(field, resultMap, fieldName, parentID)
	case reflect.Struct:
		return populateStruct(field, resultMap, fieldName)
	case reflect.Ptr:
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return populateField(field.Elem(), resultMap, fieldName, parentID)
	default:
		return populateBasicType(field, resultMap, fieldName)
	}
}

func populateBasicType(field reflect.Value, resultMap map[string]map[string][]map[string]interface{}, fullFieldName string) error {
	log.Debug().Msgf("Populating basic type fullFieldName: %s", fullFieldName)

	data, ok := resultMap[fullFieldName]
	if !ok {
		return nil // No data for this field, skip it
	}

	fieldName := strings.Split(fullFieldName, ".")[len(strings.Split(fullFieldName, "."))-1]
	log.Debug().Msgf("Populating basic type fieldName: %s", fieldName)

	for _, rows := range data {
		for _, row := range rows {
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

func populateSlice(field reflect.Value, resultMap map[string]map[string][]map[string]interface{}, fieldName string, parentID string) error {
	log.Debug().Msgf("Populating slice field: %s with parentID: %s", fieldName, parentID)
	if data, ok := resultMap[fieldName]; ok {
		rows, exists := data[parentID]
		if !exists {
			log.Debug().Msgf("No rows found for parentID: %s in field: %s", parentID, fieldName)
			return nil
		}

		for _, row := range rows {
			newElem := reflect.New(field.Type().Elem()).Elem()
			err := populateStructFromRow(newElem.Addr().Interface(), row)
			if err != nil {
				return err
			}

			// Get the ID of the new element
			newElemID := ""
			if idField := newElem.FieldByName("Id"); idField.IsValid() && idField.Kind() == reflect.Ptr && idField.Type().Elem().Kind() == reflect.String {
				if idField.Elem().IsValid() {
					newElemID = idField.Elem().String()
				}
			}

			// Recursively populate nested fields
			for i := 0; i < newElem.NumField(); i++ {
				nestedField := newElem.Field(i)
				nestedFieldName := newElem.Type().Field(i).Name
				nestedFullFieldName := fieldName + "." + nestedFieldName

				if fieldExistsInResultMap(resultMap, nestedFullFieldName) {
					err := populateField(nestedField, resultMap, nestedFullFieldName, newElemID)
					if err != nil {
						return err
					}
				}
			}

			field.Set(reflect.Append(field, newElem))
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
			return fmt.Errorf("provided value type didn't match obj field type %s and %s ", field.Type(), fieldValue.Type())
		}

		field.Set(fieldValue)
	}

	return nil
}
