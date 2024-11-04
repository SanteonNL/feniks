package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

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

	query, err := GetQueryFromFile("queries\\hix\\flat\\encounter.sql")
	//query, err := GetQueryFromFile("queries\\hix\\flat\\Observation_hix_metingen_metingen.sql")
	//query, err := GetQueryFromFile("queries\\hix\\flat\\patient.sql")

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read query from file")
	}

	dataSource := NewSQLDataSource(db, query, "Encounter", log)
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
		"Observation.code": {
			Code:  "code",
			Type:  "token",
			Value: "tyy",
		},
		"Observation.category": {
			Code:  "category",
			Type:  "token",
			Value: "tommy",
		},
	}

	// TODO: integrate with processing all json datasources
	// Load StructureDefinitions
	err = LoadStructureDefinitions(log)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load StructureDefinitions")
	}

	// Check if FhirPathToValueset is filled correctly
	for fhirPath, valueset := range FhirPathToValueset {
		log.Debug().Msgf("Path: %s, Valueset: %s\n", fhirPath, valueset)
	}

	// TODO: integrate with processing all json datasources
	// Load ConceptMaps
	err = LoadConceptMaps(log)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load ConceptMaps")
	}

	// Check if ValueSetToConceptMap is filled correctly
	for valueset, conceptMap := range ValueSetToConceptMap {
		log.Debug().Msgf("Valueset: %s, Conceptmap ID: %s\n", valueset, *conceptMap.Id)
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
