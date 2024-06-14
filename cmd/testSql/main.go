package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/SanteonNL/fenix/util"
	"github.com/gorilla/mux"

	_ "github.com/jinzhu/gorm/dialects/postgres"
)

type Endpoints struct {
	ResourceType string     `json:"resourceType"`
	SQLFile      string     `json:"sqlFile"`
	Endpoint     []Endpoint `json:"endpoint"`
}
type SearchParameter struct {
	Code       string   `json:"code"`
	Modifier   []string `json:"modifier,omitempty"`
	Comparator string   `json:"comparator,omitempty"`
	Value      string   `json:"value"`
	Type       string   `json:"type,omitempty"`
}
type Endpoint struct {
	SearchParameter []SearchParameter `json:"searchParameter"`
	SQLFile         string            `json:"sqlFile"`
}

func main() {

	r := mux.NewRouter()
	r.HandleFunc("/patients", GetAllPatients).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", r))

}

func GetAllPatients(w http.ResponseWriter, r *http.Request) {
	log.Println("GetAllPatients called")

	queryParams := r.URL.Query()
	searchParams := make([]SearchParameter, 0)

	searchParamsMap := map[string]string{
		"family":    "string",
		"birthdate": "date",
		"given":     "string",
	}

	for key, values := range queryParams {
		for _, value := range values {
			typeValue := searchParamsMap[key]
			comparator, paramValue := parseComparator(value, typeValue)

			searchParam := SearchParameter{
				Code:       key,
				Value:      paramValue,
				Type:       typeValue,
				Comparator: comparator,
			}
			searchParams = append(searchParams, searchParam)
		}
	}

	endpoints := util.GetAbsolutePath("config/endpoints2.json")

	file, err := os.Open(endpoints)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var endpoint Endpoints
	if err := json.NewDecoder(file).Decode(&endpoint); err != nil {
		log.Fatal(err)
	}

	for _, ep := range endpoint.Endpoint {
		for _, sp := range ep.SearchParameter {
			for _, param := range searchParams {
				if sp.Code == param.Code && sp.Value == param.Value {
					// Perform the desired action for matching search parameters
					// You can access the SQL file using ep.SQLFile
					fmt.Println("Matching search parameter found for SQL file:", ep.SQLFile)
				}
			}
		}
	}

	// Construct a number of FHIR patients with multiple names
	patients := []fhir.Patient{
		{Meta: &fhir.Meta{Id: util.StringPtr("id1")},
			Name: []fhir.HumanName{
				{
					Family: util.StringPtr("Hetty"),
					Given:  []string{"Robert", "Jane"},
				},
			},
			BirthDate: util.StringPtr("1990-01-01"),
		},
		{
			Name: []fhir.HumanName{
				{
					Family: util.StringPtr(""),
					Given:  []string{"Henk"},
				},
				{
					Family: util.StringPtr("Davis"),
					Given:  []string{"Emily", "Tommy"},
				},
			},
			BirthDate: util.StringPtr("1985-05-10"),
		},
	}

	// Filter patients based on search parameters
	filteredPatients := make([]fhir.Patient, 0)
	for i, patient := range patients {
		if matchesFilters(patient, searchParams) {
			log.Println("Patient", i, "matches filters")
			filteredPatients = append(filteredPatients, patient)
		}
	}

	filteredPatientsJSON, err := json.Marshal(filteredPatients)
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(filteredPatientsJSON)

}

func matchesFilters(patient fhir.Patient, filters []SearchParameter) bool {
	return checkFields(reflect.ValueOf(&patient).Elem(), filters, "")
}
func checkFields(v reflect.Value, filters []SearchParameter, parentFieldName string) bool {
	// Handle pointer dereference at the beginning
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
		if !checkFields(v, filters, parentFieldName) {
			return false
		}
	}

	if v.Kind() == reflect.String {
		fieldName := strings.ToLower(parentFieldName)
		for _, filter := range filters {
			if filter.Code == fieldName {
				log.Println("Comparing string value", v.Interface(), "with filter value", filter.Value)
				if !compare(v.Interface(), filter) {
					return false
				}
			}
		}
	}

	// Handle slices by iterating over each element
	if v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			if !checkFields(v.Index(i), filters, parentFieldName) {
				return false
			}
		}
		return true
	}

	// Handle structs by iterating over each field
	if v.Kind() == reflect.Struct {
		// if parentName == "" {
		// 	parentName = v.Type().Name()
		// }
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldName := v.Type().Field(i).Name
			//log.Println("Checking field in struct:", v.Type().Field(i).Name, "with value:", field.Interface())
			checkFields(field, filters, fieldName)
		}
	}

	return true
}

func compare(value interface{}, filter SearchParameter) bool {
	switch filter.Type {
	case "string":
		reflectedType := reflect.TypeOf(value).String()
		switch reflectedType {
		case "string":
			strValue := value.(string)
			return compareString(strValue, filter)
		case "[]string":
			log.Println("Comparing string slice value")
			return compareStringSlice(value, filter)
		default:
			return false
		}
	case "date":
		dateValue := *value.(*string)
		filterDateValue := filter.Value

		parsedDateValue, err := time.Parse("2006-01-02", dateValue)
		if err != nil {
			log.Println("Error parsing date:", err)
			return false
		}

		parsedFilterDateValue, err := time.Parse("2006-01-02", filterDateValue)
		if err != nil {
			log.Println("Error parsing filter date:", err)
			return false
		}

		switch filter.Comparator {
		case "eq", "":
			return parsedDateValue.Equal(parsedFilterDateValue)
		case "ne":
			return !parsedDateValue.Equal(parsedFilterDateValue)
		case "gt":
			return parsedDateValue.After(parsedFilterDateValue)
		case "lt":
			return parsedDateValue.Before(parsedFilterDateValue)
		case "ge":
			return parsedDateValue.After(parsedFilterDateValue) || parsedDateValue.Equal(parsedFilterDateValue)
		case "le":
			return parsedDateValue.Before(parsedFilterDateValue) || parsedDateValue.Equal(parsedFilterDateValue)
		default:
			return false
		}
	case "integer":
		// Convert patientValue and filter.Value to integers and compare them...
	default:
		return false
	}

	return false
}

func parseDateComparator(input string) (string, string) {
	comparators := []string{"eq", "ne", "gt", "lt", "ge", "le"}
	for _, comparator := range comparators {
		if strings.HasPrefix(input, comparator) {
			return comparator, strings.TrimPrefix(input, comparator)
		}
	}

	return "", input
}

func parseComparator(input string, valueType string) (comparator string, paramValue string) {
	switch valueType {
	case "string":
		return "", input
	case "date":
		comparator, value := parseDateComparator(input)
		return comparator, value
	case "integer":
		// Convert input to integer and return
		// Example: intValue, err := strconv.Atoi(input)
	default:
		return "", input
	}

	return "", input
}

func compareString(strValue string, filter SearchParameter) bool {
	filterStrValue := filter.Value
	switch filter.Comparator {
	case "eq", "":
		return strValue == filterStrValue
	case "ne":
		return strValue != filterStrValue
	default:
		return false
	}
}

func compareStringSlice(value interface{}, filter SearchParameter) bool {
	sliceValue := value.([]string)
	filterStrValue := filter.Value
	for _, strValue := range sliceValue {
		switch filter.Comparator {
		case "eq", "":
			if strValue == filterStrValue {
				log.Println("String value matched:", strValue)
				return true
			}
		case "ne":
			if strValue != filterStrValue {
				log.Println("String value not matched:", strValue)
				return true
			}
		default:
			log.Println("Invalid comparator:", filter.Comparator)
			return false
		}
	}

	log.Println("No matching string value found in slice", filterStrValue)
	return false
}
