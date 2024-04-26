package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/SanteonNL/fenix/models/sim"
)

// CSVService is a FHIRService implementation for translating CSV data to FHIR format
type CSVService struct {
	FilePath string // Path to the CSV file
}

func NewCSVService(filePath string) *CSVService {
	return &CSVService{
		FilePath: filePath,
	}
}

func main() {
	csvService := NewCSVService("input/SIMPatient.csv")

	// Get a patient record by ID
	patient, err := csvService.GetSIMPatientByID("456")
	if err != nil {
		fmt.Println("Error getting patient record:", err)
	}

	// Translate the patient record to FHIR format
	fhirPatient, err := TranslateSIMPatientToFHIR(patient)
	if err != nil {
		fmt.Println("Error translating SIM patient record:", err)
	}

	jsonBytes, err := json.Marshal(fhirPatient)
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
func (s *CSVService) GetSIMPatientByID(id string) (*sim.Patient, error) {
	file, err := os.Open(s.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'

	// Read and discard the header row
	if _, err := reader.Read(); err != nil {
		return nil, err
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		geboortedatum, _ := time.Parse("2006-01-02", record[6])
		patient := &sim.Patient{
			Identificatienummer: &record[0],
			Geboortedatum:       &geboortedatum, // Parse the BirthDate field as a time.Time,
			// Continue for the rest of the fields...
		}

		if *patient.Identificatienummer == id {
			return patient, nil
		}
	}

	return nil, fmt.Errorf("no patient found with ID %s", id)
}

func TranslateSIMPatientToFHIR(patient *sim.Patient) (*fhir.Patient, error) {
	if patient == nil {
		return nil, errors.New("nil patient record")
	}

	// Translate to FHIR format
	birthDate := patient.Geboortedatum.Format("2006-01-02") // Format the BirthDate field as a string
	fhirPatient := &fhir.Patient{
		Id:        patient.Identificatienummer,
		BirthDate: &birthDate,
	}

	return fhirPatient, nil
}
