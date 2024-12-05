package structuredefinition

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
)

// StructureDefinitionRepository handles loading and storing StructureDefinition resources.
type StructureDefinitionRepository struct {
	log                     zerolog.Logger
	structureDefinitionsMap map[string]fhir.StructureDefinition
}

// NewStructureDefinitionRepository creates a new StructureDefinitionRepository.
func NewStructureDefinitionRepository(log zerolog.Logger) *StructureDefinitionRepository {
	return &StructureDefinitionRepository{
		log:                     log,
		structureDefinitionsMap: make(map[string]fhir.StructureDefinition),
	}
}

// LoadStructureDefinitions loads all StructureDefinitions into the repository.
func (repo *StructureDefinitionRepository) LoadStructureDefinitions(path string) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read StructureDefinitions directory: %v", err)
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			filePath := filepath.Join(path, file.Name())
			structureDefinition, err := ReadFHIRResource[fhir.StructureDefinition](filePath, fhir.UnmarshalStructureDefinition)
			if err != nil {
				return fmt.Errorf("failed to read StructureDefinition from file: %v", err)
			}
			repo.structureDefinitionsMap[structureDefinition.Name] = *structureDefinition
			repo.log.Debug().Str("structureDefinition", file.Name()).Msg("Loaded StructureDefinition")
		}
	}

	return nil
}

// GetStructureDefinition retrieves a StructureDefinition by name.
func (repo *StructureDefinitionRepository) GetStructureDefinition(name string) (*fhir.StructureDefinition, error) {
	structureDefinition, exists := repo.structureDefinitionsMap[name]
	if !exists {
		return nil, fmt.Errorf("StructureDefinition not found: %s", name)
	}
	return &structureDefinition, nil
}

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
