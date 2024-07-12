package main

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type RowData map[string]interface{}

func main() {

	l := zerolog.New(zerolog.MultiLevelWriter(zerolog.ConsoleWriter{Out: os.Stdout})).With().Timestamp().Caller().Logger()

	l.Debug().Msg("Starting flatSqlToJson")

	db, err := sqlx.Connect("postgres", "postgres://postgres:mysecretpassword@localhost:5432/tsl_employee?sslmode=disable")
	if err != nil {
		fmt.Println(err)
		return
	}

	patient := fhir.Patient{}

	err = PopulateStructs(db, &patient)
	if err != nil {
		fmt.Println(err)
		return
	}

	//fmt.Printf("%+v\n", patient)

}

func PopulateStructs(db *sqlx.DB, s interface{}) error {
	query := `
    SELECT
    'Patient' as field_name,
    '' as parent_id,
    p.identificatienummer id,
    -- CASE
    --     WHEN p.geslachtcode = 'M' THEN 'male'
    --     WHEN p.geslachtcode = 'F' THEN 'female'
    --     ELSE 'unknown'
    -- END as gender, 
    p.geboortedatum birthdate
    FROM patient p
WHERE 1=1
AND p.identificatienummer = '123';


SELECT
    'Patient.Name' as field_name,
    p.identificatienummer as parent_id,
    concat(p.identificatienummer,humanName.lastname) AS id,
    humanName.lastname as family,
    humanName.firstname AS name
FROM
    patient p
    JOIN names humanName ON humanName.identificatienummer = p.identificatienummer
WHERE 1=1
 AND p.identificatienummer = '123'   
GROUP BY
    p.identificatienummer, humanName.lastname,humanName.firstname;
   `

	//patientID := "123"
	rows, err := db.Queryx(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	resultMap := make(map[string]map[string][]RowData)

	for {
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			if !rows.NextResultSet() {
				break
			}
			continue
		}

		// Scan the entire row into a map
		rowData := make(RowData)
		err = rows.MapScan(rowData)
		if err != nil {
			return err
		}

		fieldName := rowData["field_name"].(string)
		parentID := rowData["parent_id"].(string)

		if _, ok := resultMap[fieldName]; !ok {
			resultMap[fieldName] = make(map[string][]RowData)
		}
		resultMap[fieldName][parentID] = append(resultMap[fieldName][parentID], rowData)

		//	log.Debug().Msgf("fieldName: %s, parentID: %s, rowData: %v", fieldName, parentID, rowData)
	}

	err = populateStruct(s, resultMap, "")
	if err != nil {
		return err
	}

	return nil
}

func populateStruct(s interface{}, resultMap map[string]map[string][]RowData, parentPath string) error {
	v := reflect.ValueOf(s).Elem()
	t := v.Type()

	structName := reflect.TypeOf(s).Elem().Name()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldName := field.Name

		if parentPath == "" {
			parentPath = structName
		}

		//fieldPath := parentPath + "." + fieldName

		//log.Debug().Msgf("fieldName before if: %s", fieldName)

		if data, ok := resultMap[structName]; ok {
			//log.Debug().Msgf("fieldPath: %s", fieldPath)

			parentID := ""
			if idField := v.FieldByName("ID"); idField.IsValid() {
				parentID = idField.String()
			}

			log.Debug().Msgf("parentID: %s, data[] %v", parentID, data[parentID])

			if rowsSlice, ok := data[parentID]; ok && len(rowsSlice) == 1 {
				fieldValue := v.Field(i)
				log.Debug().Msgf("fieldName %s, fieldValue: %v, rowSlice %v", fieldName, fieldValue, rowsSlice)
				err := populateStructFromRow(fieldValue, fieldName, rowsSlice[0])
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func populateStructFromRow(fieldValue reflect.Value, fieldName string, row RowData) error {
	for key, value := range row {
		log.Debug().Msgf("key: %s, fieldName %s, value: %v", key, fieldName, value)
		fieldNameLower := strings.ToLower(fieldName)
		if fieldNameLower == key {
			log.Debug().Msgf("Setting value for field %s: %v", fieldName, value)
			if fieldValue.Kind() == reflect.Ptr {
				fieldValue = fieldValue.Elem()
			}
			if field := fieldValue.FieldByName(key); field.IsValid() && field.CanSet() {
				err := setFieldValue(field, value)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func setFieldValue(field reflect.Value, value interface{}) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value.(string))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intValue, err := strconv.ParseInt(value.(string), 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(intValue)
	case reflect.Float32, reflect.Float64:
		floatValue, err := strconv.ParseFloat(value.(string), 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatValue)
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value.(string))
		if err != nil {
			return err
		}
		field.SetBool(boolValue)
	default:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	}
	return nil
}
