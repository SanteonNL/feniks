package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

func main() {
	startTime := time.Now()
	log := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) { w.Out = os.Stdout })).With().Timestamp().Caller().Logger()
	log.Debug().Msg("Starting fenix")

	db, err := sqlx.Connect("postgres", "postgres://postgres:mysecretpassword@localhost:5432/tsl_employee?sslmode=disable")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to the database")
	}
	defer db.Close()

	query, err := GetQueryFromFile("queries\\hix\\flat\\Observation_hix_metingen_metingen.sql")
	//query, err := GetQueryFromFile("queries\\hix\\flat\\encounter.sql")
	//query, err := GetQueryFromFile("queries\\hix\\flat\\Observation_hix_metingen_metingen.sql")
	//query, err := GetQueryFromFile("queries\\hix\\flat\\patient.sql")

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read query from file")
	}
	resourceType := "Observation"
	dataSource := NewSQLDataSource(db, query, resourceType, log)
	//dataSource := NewSQLDataSource(db, query, "Encounter", log)
	//dataSource := NewSQLDataSource(db, query, "Observation", log)
	//dataSource := NewSQLDataSource(db, query, "Patient", log)
	// Setup search parameters
	searchParams := SearchParameterMap{
		// "Patient.identifier": {
		// 	Code:  "category",
		// 	Type:  "token",
		// 	Value: "1sas",
		// },
		// "Patient.birthdate": {
		// 	Code:       "birthdate",
		// 	Type:       "date",
		// 	Comparator: "eq",
		// 	Value:      "1992-01-01",
		// },
		"Observation.category": {
			Code:  "category",
			Type:  "token",
			Value: "https://decor.nictiz.nl/fhir/4.0/san-gen-/ValueSet/2.16.840.1.113883.2.4.3.11.60.124.11.115--20240819114333?_format=json",
		},
		// "Observation.category": {
		// 	Code:  "category",
		// 	Type:  "token",
		// 	Value: "tommy",
		// },
	}

	// TODO: integrate with processing all json datasources
	// Load StructureDefinitions
	err = LoadStructureDefinitions(log)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load StructureDefinitions")
	}

	// You can also update bindings separately if needed:
	UpdateSearchParameterBindings(resourceType, searchParams, log)

	// Check if FhirPathToValueset is filled correctly
	for fhirPath, valueset := range FhirPathToValueset {
		log.Debug().Str("Path", fhirPath).Str("ValueSet", valueset).Msgf("Check if FhirPathToValueset is filled correctly")
	}

	// Create a new ResourceLoader
	rl := NewResourceLoader("config", log)

	// Load resources
	if err := rl.LoadResources(); err != nil {
		log.Fatal().Err(err).Msg("Failed to load resources")
	}

	// Fix ConceptMaps
	if err := rl.FixConceptMaps(); err != nil {
		log.Fatal().Err(err).Msg("Failed to fix ConceptMaps")
	}

	// TODO: integrate with processing all json datasources
	// Load ConceptMaps
	err = LoadConceptMaps(log)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load ConceptMaps")
	}

	// Initialize the cache
	cache := NewValueSetCache("./valuesets", log)

	// Create a coding to validate
	coding := &fhir.Coding{
		System: ptr("http://snomed.info/sct"),
		Code:   ptr("260686004"),
	}

	// Validate the code
	result, err := cache.ValidateCode("https://decor.nictiz.nl/fhir/4.0/sansa-/ValueSet/2.16.840.1.113883.2.4.3.11.60.909.11.2--20241203090354", coding)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	if result.Valid {
		fmt.Printf("Code is valid! Found in: %s\n", result.MatchedIn)
	} else {
		fmt.Printf("Code is invalid: %s\n", result.ErrorMessage)
	}

	// inputConceptMapFile, err := filepath.Abs("config\\conceptmaps\\flat\\conceptmap_TommyMeetMethodeLijst.csv")
	// if err != nil {
	// 	log.Fatal().Err(err).Msg("Failed to get absolute path for input concept map file")
	// }
	// outputConceptMapFile, err := filepath.Abs("config\\conceptmaps\\flat\\conceptmap_TommyMeetMethodeLijst_validated.csv")
	// if err != nil {
	// 	log.Fatal().Err(err).Msg("Failed to get absolute path for output concept map file")
	// }

	// // err = ValidateCSVMappings(inputConceptMapFile, outputConceptMapFile, cache, log)
	// // if err != nil {
	// // 	log.Fatal().Err(err).Msg("Validation of conceptmap failed")
	// // }

	// Create converter
	converter := NewConceptMapConverter(cache, log)

	// Basic usage - will use default "active" status
	err = converter.ConvertToFHIR(
		"config/conceptmaps/flat/conceptmap_TommyMeetMethodeLijst_validated.csv",
		"config/conceptmaps/fhir/conceptmap_converted.json",
	)
	// Basic usage - will use default "active" status
	err = converter.ConvertToFHIR(
		"config/conceptmaps/flat/conceptmap_TommyMeetMethodeLijst_validated.csv",
		"config/conceptmaps/fhir/conceptmap_converted.json",
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Conversion failed")
	}

	if err != nil {
		log.Fatal().Err(err).Msg("Conversion failed")
	}

	os.Exit(0)
	// Check if ValueSetToConceptMap is filled correctly
	for valuesetName, conceptMap := range ValueSetToConceptMap {
		log.Debug().Msgf("Valueset: %s, Conceptmap ID: %s\n", valuesetName, *conceptMap.Id)
	}

	// Process resources
	resources, err := ProcessResources(dataSource, "12345", searchParams, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to process resources")
	}

	// Output results
	for _, resource := range resources {
		if jsonData, err := json.MarshalIndent(resource, "", "  "); err == nil {
			fmt.Println(string(jsonData))
		}
	}

	outputDir := "output/temp"
	if err := WriteToJSON(resources, "resources", outputDir, log); err != nil {
		log.Error().Err(err).Msg("Failed to write raw results")
		// Continue processing despite write error
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	log.Debug().Msgf("Execution time: %s", duration)
}
