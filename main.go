package main

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/SanteonNL/fenix/models/sim"
	"github.com/gorilla/mux"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

type Config struct {
	Services []ServiceConfig `json:"services"`
}

type ServiceConfig struct {
	Type         string `json:"type"`
	Format       string `json:"format"`
	DatabaseType string `json:"databaseType"`
	ConnStr      string `json:"connStr"`
	SourcePath   string `json:"sourcePath"`
}

func NewService(config ServiceConfig) (Service, error) {
	switch config.Type {
	// case "postgres":
	// 	//return NewPostgreSQLService(config.ConnStr)
	// case "sqlserver":
	// 	//return NewSQLServerService(config.ConnStr)
	case "csv":
		switch config.Format {
		case "sim":
			return NewSIMCSVService(config.SourcePath), nil
		// case "fhir":
		// 	//return NewFHIRCSVService(config.FilePath), nil
		// case "castor":
		// 	// Create a Castor CSV service...
		default:
			return nil, fmt.Errorf("unsupported CSV format: %s", config.Format)
		}
	case "ndjson":
		switch config.Format {
		case "fhir":
			return NewFHIRNDJSONService(config.SourcePath), nil
		// case "castor":
		// 	// Create a Castor NDJSON service...
		default:
			return nil, fmt.Errorf("unsupported NDJSON format: %s", config.Format)
		}
	case "sql":
		switch config.DatabaseType {
		case "postgres":
			return NewSQLService(config.ConnStr, config.DatabaseType)
		// case "sqlserver":
		// 	//return NewSQLServerService(config.ConnStr)
		default:
			return nil, fmt.Errorf("unsupported database type: %s", config.DatabaseType)
		}
	default:
		return nil, fmt.Errorf("unsupported service type: %s", config.Type)
	}
}

// CSVService is a FHIRService implementation for translating CSV data to FHIR format
type SIMCSVService struct {
	FilePath string // Path to the CSV file
}

func NewSIMCSVService(filePath string) *SIMCSVService {
	return &SIMCSVService{
		FilePath: filePath,
	}
}

type Service interface {
	GetPatient(id string) (*fhir.Patient, error)
	GetAllPatients() ([]*fhir.Patient, error)
}

// type PatientService interface {
// 	GetPatient(id string) (*fhir.Patient, error)
// 	GetAllPatients() ([]*fhir.Patient, error)
// }

type Application struct {
	Services []Service
}

func main() {

	file, err := os.Open("config/sources.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		log.Fatal(err)
	}

	app := &Application{
		Services: []Service{},
	}

	for _, serviceConfig := range config.Services {
		service, err := NewService(serviceConfig)
		if err != nil {
			log.Fatal(err)
		}
		app.Services = append(app.Services, service)
	}

	r := mux.NewRouter()
	r.HandleFunc("/patient/{id}", app.GetPatient).Methods("GET")
	r.HandleFunc("/patients/{id}", app.GetAllPatients).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", r))

}

func (app *Application) GetPatient(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	for _, service := range app.Services {
		patient, err := service.GetPatient(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		jsonBytes, err := json.Marshal(patient)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write(jsonBytes)
		return
	}

	http.Error(w, "Patient not found", http.StatusNotFound)
}

func (app *Application) GetAllPatients(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("Content-Type", "application/fhir+ndjson")

	var allPatients []*fhir.Patient
	for _, service := range app.Services {
		patients, err := service.GetAllPatients()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		allPatients = append(allPatients, patients...)
	}

	jsonBytes, err := json.Marshal(allPatients)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonBytes)
}

// type fhirResource interface {
// 	MarshalJSON() ([]byte, error)
// }

//	func (s *SIMCSVService) GetResource(resourceType string, id string) ([]fhirResource, error) {
//		switch resourceType {
//		case "patient":
//			SIMPatient, err := s.GetPatient(id)
//			if err != nil {
//				return nil, err
//			}
//			fhirPatient, err := TranslateSIMPatientToFHIR(SIMPatient)
//			if err != nil {
//				return nil, err
//			}
//			return []fhirResource{fhirPatient}, nil
//		// Add other cases for other resource types...
//		case "patient/$export":
//			patients, err := s.GetAllPatients()
//			if err != nil {
//				return nil, err
//			}
//			var fhirPatients []fhirResource
//			for _, patient := range patients {
//				fhirPatient, err := TranslateSIMPatientToFHIR(patient)
//				if err != nil {
//					return nil, err
//				}
//				fhirPatients = append(fhirPatients, fhirPatient)
//			}
//			return fhirPatients, nil
//		default:
//			return nil, fmt.Errorf("resource type %s is not supported", resourceType)
//		}
//	}

func (s *SIMCSVService) readCSVFile() (*os.File, *csv.Reader, error) {
	file, err := os.Open(s.FilePath)
	if err != nil {
		return nil, nil, err
	}

	reader := csv.NewReader(file)
	reader.Comma = ';'

	// Read and discard the header row
	if _, err := reader.Read(); err != nil {
		return nil, nil, err
	}

	return file, reader, nil
}

func (s *SIMCSVService) GetPatient(id string) (*fhir.Patient, error) {
	file, reader, err := s.readCSVFile()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		simPatient, err := mapRecordToSIMPatient(record)
		if err != nil {
			return nil, err
		}

		fhirPatient, err := TranslateSIMPatientToFHIR(simPatient)
		if err != nil {
			return nil, err
		}

		if *fhirPatient.Id == id {
			return fhirPatient, nil
		}
	}

	return nil, nil
}

func (s *SIMCSVService) GetAllPatients() ([]*fhir.Patient, error) {
	file, reader, err := s.readCSVFile()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patients []*fhir.Patient
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		simPatient, err := mapRecordToSIMPatient(record)
		if err != nil {
			return nil, err
		}

		fhirPatient, err := TranslateSIMPatientToFHIR(simPatient)
		if err != nil {
			return nil, err
		}

		patients = append(patients, fhirPatient)
	}

	if len(patients) == 0 {
		return nil, fmt.Errorf("no patients found")
	}

	return patients, nil
}

func mapRecordToSIMPatient(record []string) (*sim.Patient, error) {
	if len(record) < 2 {
		return nil, fmt.Errorf("invalid record format")
	}

	patient := &sim.Patient{
		Identificatienummer: &record[0],
		Geboortedatum:       parseTime(&record[1]),
		// Continue mapping the rest of the fields...
	}

	return patient, nil
}

func TranslateSIMPatientToFHIR(patient *sim.Patient) (*fhir.Patient, error) {
	fhirPatient := &fhir.Patient{
		Id:        patient.Identificatienummer,
		BirthDate: toString(patient.Geboortedatum),
	}

	return fhirPatient, nil
}

func toString(time *time.Time) *string {
	str := time.Format("2006-01-02")
	return &str
}

type FHIRNDJSONService struct {
	FilePath string // Path to the NDJSON file
}

func NewFHIRNDJSONService(filePath string) *FHIRNDJSONService {
	return &FHIRNDJSONService{
		FilePath: filePath,
	}
}

// func (s *FHIRNDJSONService) GetResource(resourceType string, id string) ([]fhirResource, error) {
// 	switch resourceType {
// 	case "patient":
// 		return s.GetPatient(id)
// 		// Add other cases for other resource types...
// 	case "patient/$export":
// 		return s.GetAllPatients(id)
// 		// Add other cases for other resource types...
// 	default:
// 		return nil, fmt.Errorf("resource type %s is not supported", resourceType)
// 	}
// }

func (s *FHIRNDJSONService) GetPatient(id string) (*fhir.Patient, error) {
	file, err := os.Open(s.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		var patient fhir.Patient
		if err := json.Unmarshal([]byte(line), &patient); err != nil {
			return nil, err
		}

		if *patient.Id == id {
			return &patient, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *FHIRNDJSONService) GetAllPatients() ([]*fhir.Patient, error) {
	file, err := os.Open(s.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var patients []*fhir.Patient

	for scanner.Scan() {
		line := scanner.Text()

		var patient fhir.Patient
		if err := json.Unmarshal([]byte(line), &patient); err != nil {
			return nil, err
		}

		patients = append(patients, &patient)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patients, nil
}

func TranslateFHIRPatientToSIM(patient *fhir.Patient) (*sim.Patient, error) {
	simPatient := &sim.Patient{
		Identificatienummer: patient.Id,
		Geboortedatum:       parseTime(patient.BirthDate),
		// Continue for the rest of the fields...
	}

	return simPatient, nil
}

func parseTime(s *string) *time.Time {
	if s == nil {
		return nil
	}
	t, _ := time.Parse("2006-01-02", *s)
	return &t
}

type SQLService struct {
	db *gorm.DB
}

func NewSQLService(connStr string, databaseType string) (*SQLService, error) {
	db, err := gorm.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return &SQLService{db: db}, nil
}

func (s *SQLService) GetPatient(id string) (*fhir.Patient, error) {
	var patient sim.Patient
	err := s.db.Raw("SELECT * FROM patient_hix_patient WHERE identificatienummer = ?", id).Scan(&patient).Error
	if err != nil {
		return nil, err
	}

	fhirPatient, err := TranslateSIMPatientToFHIR(&patient)
	if err != nil {
		return nil, err
	}

	return fhirPatient, nil
}
func (s *SQLService) GetAllPatients() ([]*fhir.Patient, error) {

	query, err := os.ReadFile("queries/hix/patient.sql")
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Raw(string(query)).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fhirPatients []*fhir.Patient

	for rows.Next() {
		var patientJSON sql.NullString
		err := rows.Scan(&patientJSON)
		if err != nil {
			return nil, err
		}

		var patient fhir.Patient
		err = json.Unmarshal([]byte(patientJSON.String), &patient)
		if err != nil {
			return nil, err
		}

		fhirPatients = append(fhirPatients, &patient)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return fhirPatients, nil
}

// func (s *SQLService) GetResource(resourceType string, id string) (fhirResource, error) {
// 	switch resourceType {
// 	case "patient":
// 		SIMPatient, err := s.GetPatient(id)
// 		if err != nil {
// 			return nil, err
// 		}
// 		fhirPatient, err := TranslateSIMPatientToFHIR(SIMPatient)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return fhirPatient, nil
// 	default:
// 		return nil, fmt.Errorf("resource type %s is not supported", resourceType)
// 	}
// }

// 	case "patient":
// 		SIMPatient, err := s.GetPatient(id)
// 		if err != nil {
// 			return nil, err
// 		}
// 		fhirPatient, err := TranslateSIMPatientToFHIR(SIMPatient)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return fhirPatient, nil
// 	default:
// 		return nil, fmt.Errorf("resource type %s is not supported", resourceType)
// 	}
// }
