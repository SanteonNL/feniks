package main

import (
	"fmt"
	"os"
	"time"

	"github.com/SanteonNL/fenix/cmd/fenix/datasource"
	"github.com/SanteonNL/fenix/cmd/fenix/output"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

func main() {
	startTime := time.Now()

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	outputMgr, err := output.NewOutputManager("output/temp", log)

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create output manager")
	}

	log = outputMgr.GetLogger()

	log.Debug().Msg("Starting fenix")

	// Th	// Initialize database connection
	db, err := sqlx.Connect("postgres", "postgres://postgres:mysecretpassword@localhost:5432/tsl_employee?sslmode=disable")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	service := datasource.NewDataSourceService(db, log)

	// Load queries
	err = service.LoadQueryFile("queries/hix/flat/patient_1.sql")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load query file")
	}

	// Read resources
	results, err := service.ReadResources("Patient", "12345")
	if err != nil {
		log.Printf("Error: %v", err)
	}

	// Print results
	for _, result := range results {
		for path, rowData := range result {
			fmt.Printf("Resource Path: %s, Data: %v\n", path, rowData)
		}
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	log.Debug().Msgf("Execution time: %s", duration)
}
