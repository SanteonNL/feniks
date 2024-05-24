package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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

	queryParams := r.URL.Query()
	searchParams := make([]SearchParameter, 0)

	for key, values := range queryParams {
		for _, value := range values {
			searchParam := SearchParameter{
				Code:  key,
				Value: value,
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

	w.Write([]byte("Hello World"))

}
