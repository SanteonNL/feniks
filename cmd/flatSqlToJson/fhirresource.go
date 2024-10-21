package main

import (
	"fmt"
	"io"
	"os"
)

// UnmarshalFunc is a function type for unmarshalling FHIR resources.
type UnmarshalFunc[T any] func([]byte) (T, error)

// ReadFHIRResource reads a FHIR resource from a JSON file and unmarshals it using the provided unmarshal function.
func ReadFHIRResource[T any](filePath string, unmarshal UnmarshalFunc[T]) (*T, error) {
	// Open the JSON file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Read the file content
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Unmarshal the JSON data using the provided unmarshal function
	resource, err := unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource: %v", err)
	}

	return &resource, nil
}
