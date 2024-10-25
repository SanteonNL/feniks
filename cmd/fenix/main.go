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

	query, err := GetQueryFromFile("queries\\hix\\flat\\patient_index.sql")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read query from file")
	}

	dataSource := NewSQLDataSource(db, query, "Patient", log)
	// Setup search parameters
	searchParams := SearchParameterMap{
		"Patient.birthdate": {
			Code:       "birthdate",
			Type:       "date",
			Value:      "1990-01-01",
			Comparator: "le",
		},
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

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	log.Debug().Msgf("Execution time: %s", duration)
}
