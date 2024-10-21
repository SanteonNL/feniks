package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
)

// StructureDefinitionsMap stores all structure definitions.
var StructureDefinitionsMap = make(map[string]fhir.StructureDefinition)

// LoadStructureDefinitions loads all StructureDefinitions into a global map.
func LoadStructureDefinitions(log zerolog.Logger) error {
	files, err := os.ReadDir("structuredefinitions")
	if err != nil {
		return fmt.Errorf("failed to read StructureDefinitions directory: %v", err)
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			filePath := filepath.Join("structuredefinitions", file.Name())
			structureDefinition, err := ReadFHIRResource(filePath, fhir.UnmarshalStructureDefinition)
			if err != nil {
				return fmt.Errorf("failed to read StructureDefinition from file: %v", err)
			}
			StructureDefinitionsMap[structureDefinition.Name] = *structureDefinition
			log.Debug().Str("structureDefinition", file.Name()).Msg("Loaded StructureDefinition")
			PrintElementsWithCodeType(structureDefinition)
		}
	}

	return nil
}
