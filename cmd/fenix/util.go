package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/rs/zerolog"
)

func getStringValue(field reflect.Value) string {
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return ""
		}
		field = field.Elem()
	}
	return field.String()
}

// GetQueryFromFile reads a SQL query from a file and returns it as a string
func GetQueryFromFile(relativePath string) (string, error) {
	queryPath, err := filepath.Abs(relativePath)
	if err != nil {
		return "", err
	}

	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		return "", err
	}
	return string(queryBytes), nil
}

// WriteToJSON is a generic function that writes any data to a JSON file
// It takes a prefix to identify different types of intermediary results
func WriteToJSON[T any](data T, prefix string, outputDir string, log zerolog.Logger) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate timestamp for unique filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.json", prefix, timestamp)
	outputPath := filepath.Join(outputDir, filename)

	// Create the file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Create an encoder with indentation for readable output
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	// Write the data
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode data to JSON: %w", err)
	}

	log.Debug().
		Str("file", outputPath).
		Str("prefix", prefix).
		Msg("Wrote intermediary results to JSON file")

	return nil
}
