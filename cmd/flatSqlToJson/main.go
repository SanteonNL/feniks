package main

import (
	"fmt"
	"os"
	"reflect"
	"strconv"

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

	err = populateStruct(s, resultMap, "Patient")
	if err != nil {
		return err
	}

	return nil
}

func populateStruct(s interface{}, resultMap map[string]map[string][]RowData, parentPath string) error {
	v := reflect.ValueOf(s).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldName := field.Name
		if parentPath != "" {
			fieldName = parentPath + "." + fieldName
		}

		if data, ok := resultMap[fieldName]; ok {
			log.Debug().Msgf("fieldName: %s", fieldName)

			parentID := ""
			if idField := v.FieldByName("ID"); idField.IsValid() {
				parentID = idField.String()
			}

			log.Debug().Msgf("parentID: %s, data[] %v", parentID, data["123"])

			if rowsSlice, ok := data[parentID]; ok {
				fieldValue := v.Field(i)
				log.Debug().Msgf("fieldValue: %v, rowSlice %v", fieldValue, rowsSlice)

				if field.Type.Kind() == reflect.Struct {
					// Recursively populate nested structs
					err := populateStruct(fieldValue.Addr().Interface(), resultMap, fieldName)
					if err != nil {
						return err
					}
				} else if field.Type.Kind() == reflect.Slice {
					// Handle slices
					sliceType := field.Type.Elem()
					slice := reflect.MakeSlice(field.Type, len(rowsSlice), len(rowsSlice))

					for j, row := range rowsSlice {
						elem := reflect.New(sliceType).Elem()
						err := populateStructFromRow(elem, row)
						if err != nil {
							return err
						}
						slice.Index(j).Set(elem)
					}

					fieldValue.Set(slice)
				} else {
					// Handle simple fields
					if len(rowsSlice) > 0 {
						err := populateStructFromRow(fieldValue, rowsSlice[0])
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func populateStructFromRow(fieldValue reflect.Value, row RowData) error {
	for key, value := range row {
		if field := fieldValue.FieldByName(key); field.IsValid() && field.CanSet() {
			err := setFieldValue(field, value)
			if err != nil {
				return err
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
