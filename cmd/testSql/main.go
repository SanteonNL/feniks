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
	log.Println("Server started on port 8080")
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
			param := SearchParameter{
				Code:       key,
				Value:      paramValue,
				Type:       typeValue,
				Comparator: comparator,
			}
			searchParams = append(searchParams, param)
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
					Family: util.StringPtr("Smith"),
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
			log.Println("Patient", i, "matches filters \n")
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
	return checkFields(reflect.ValueOf(&patient).Elem(), filters, "Patient")
}
func checkFields(v reflect.Value, filters []SearchParameter, parentField string) bool {
	// TODO check if rellect.Value is a struct

	matchFound := false // Track if any field matches

	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldName := v.Type().Field(i).Name
			fieldName = parentField + "." + fieldName

			log.Println("Checking field:", fieldName, "with type:", field.Type().String(), "and kind:", field.Kind())

			if field.Kind() == reflect.Slice {
				for i := 0; i < field.Len(); i++ {
					element := field.Index(i)
					log.Println("Checking kind of element:", element.Kind())
					if element.Kind() == reflect.Struct {
						if checkFields(element, filters, fieldName) {
							matchFound = true
						}
					} else if element.Kind() == reflect.String {
						strValue := element.Interface().(string)
						log.Println("Checking string value in []string", strValue)
						for _, filter := range filters {
							if compareString(strValue, filter) {
								matchFound = true
								break // Match found, no need to check further
							}
						}
					}
				}
			}
			if field.Kind() == reflect.Struct {
				log.Println("Checking nested struct")
				if checkFields(field, filters, fieldName) {
					matchFound = true
				}
			} else if field.Kind() == reflect.Ptr && field.Elem().Kind() == reflect.Struct {
				log.Println("Checking pointer to struct")
				if checkFields(field.Elem(), filters, fieldName) {
					matchFound = true
				}
			} else if field.Kind() == reflect.Ptr && field.Elem().Kind() == reflect.String {
				ptrValue := field.Interface().(*string)
				if ptrValue != nil {
					strValue := *ptrValue
					log.Println("Checking string value:", strValue)
					for _, filter := range filters {
						log.Println("Comparing string value:", strValue, "with filter:", filter.Value)
						if compareString(strValue, filter) {
							log.Println("Match found in string value")
							matchFound = true
							break // Match found, no need to check further
						}
					}
				}
			}
		}
	}

	if !matchFound {
		log.Println("No match found in any field")
		return false
	}

	log.Println("Match found in struct")
	return true // Match found
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
