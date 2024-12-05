package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/SanteonNL/fenix/cmd/fenix/fhir/conceptmap"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/structuredefinition"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/valueset"
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
	cache := valueset.NewValueSetCache("./valuesets", log)

	// Create a coding to validate
	coding := &fhir.Coding{
		System: ptr("http://snomed.info/sct"),
		Code:   ptr("260686004"),
	}

	// Create a context
	ctx := context.Background()

	// Validate the code
	result, err := cache.ValidateCode(ctx, "https://decor.nictiz.nl/fhir/4.0/sansa-/ValueSet/2.16.840.1.113883.2.4.3.11.60.909.11.2--20241203090354", coding)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	if result.Valid {
		fmt.Printf("Code is valid! Found in: %s\n", result.MatchedIn)
	} else {
		fmt.Printf("Code is invalid: %s\n", result.ErrorMessage)
	}

	// Initialize the repository and service
	conceptmapRepo, err := conceptmap.NewConceptMapRepository("config/conceptmaps/fhir", log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize ConceptMap repository")
	}

	// Initialize ConceptMap service with the new valueset cache
	conceptMapService := conceptmap.NewConceptMapService(conceptmapRepo, log)

	// Initialize ConceptMap converter
	converter := conceptmap.NewConceptMapConverter(log, conceptMapService)
	// Basic usage - will use default "active" status
	// Convert CSV to FHIR ConceptMap
	inputFile := "config/conceptmaps/flat/conceptmap_TommyMeetMethodeLijst_validated.csv"
	file, err := os.Open(inputFile)
	if err != nil {
		log.Fatal().Err(err).Str("file", inputFile).Msg("Failed to open input file")
	}
	defer file.Close()

	// Convert the CSV to a FHIR ConceptMap using the new converter
	conceptMapFHIR, err := converter.ConvertCSVToFHIR(file, "TommyMeetMethodeLijst")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to convert CSV to FHIR ConceptMap")
	}

	// Save the converted ConceptMap using the service
	outputPath := "config/conceptmaps/fhir/conceptmap_converted2.json"
	if err := conceptMapService.SaveConceptMap(outputPath, conceptMapFHIR); err != nil {
		log.Fatal().Err(err).Msg("Failed to save ConceptMap")
	}
	// Initialize the repository and service
	conceptStructureRepo := structuredefinition.NewStructureDefinitionRepository(log)
	service := structuredefinition.NewStructureDefinitionService(conceptStructureRepo, log)

	// Load StructureDefinitions
	err = conceptStructureRepo.LoadStructureDefinitions("profiles/sim")
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	// Example usage: Get ValueSet binding for a specific path
	path := "Observation.code"
	valueSetURL, err := service.GetValuesetBindingForPath(path)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	fmt.Printf("ValueSet URL for path %s: %s\n", path, valueSetURL)

	conceptMap, err := conceptMapService.GetConceptMapsByValuesetURL(valueSetURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get ConceptMap for ValueSet")
	}
	for _, cm := range conceptMap {
		fmt.Printf("Found ConceptMap: %s\n", *cm.Name)
	}

	//os.Exit(0)
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
