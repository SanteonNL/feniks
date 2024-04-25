package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/SanteonNL/fenix/fhir"
)

// CSVService is a FHIRService implementation for translating CSV data to FHIR format
type CSVService struct {
	FilePath string // Path to the CSV file
}

// NewCSVService creates a new CSVService with the provided file path
func NewCSVService(filePath string) *CSVService {
	return &CSVService{
		FilePath: filePath,
	}
}

func main() {
	// Create a new patient struct
	id := "123"
	firstName := "John"
	lastName := "Doe"
	birthDate := "2020-01-01"

	patient := fhir.Patient{
		Id:        &id,
		Name:      []fhir.HumanName{{Given: []string{firstName}, Family: &lastName}},
		BirthDate: &birthDate,
	}

	jsonBytes, err := json.Marshal(patient)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonBytes, "", "\t")
	if err != nil {
		fmt.Println("Error formatting JSON:", err)
		return
	}

	fmt.Println(prettyJSON.String())
}
