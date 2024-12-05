package structuredefinition

import (
	"fmt"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
)

// StructureDefinitionService provides functionality to interact with StructureDefinition resources.
type StructureDefinitionService struct {
	repo *StructureDefinitionRepository
	log  zerolog.Logger
}

// NewStructureDefinitionService creates a new StructureDefinitionService.
func NewStructureDefinitionService(repo *StructureDefinitionRepository, log zerolog.Logger) *StructureDefinitionService {
	return &StructureDefinitionService{
		repo: repo,
		log:  log,
	}
}

// CollectValuesetBindingsForCodeTypes collects elements from the StructureDefinition with code types and their value set bindings.
func (svc *StructureDefinitionService) CollectValuesetBindingsForCodeTypes(structureDefinition *fhir.StructureDefinition) map[string]string {
	bindings := make(map[string]string)
	for _, element := range structureDefinition.Snapshot.Element {
		for _, t := range element.Type {
			if t.Code == "code" || t.Code == "Coding" || t.Code == "CodeableConcept" || t.Code == "Quantity" {
				if element.Binding != nil {
					bindings[element.Path] = *element.Binding.ValueSet
				} else {
					svc.log.Debug().Msgf("No binding for path: %s, code: %s", element.Path, t.Code)
				}
				break
			}
		}
	}
	return bindings
}

// GetValuesetBindingForPath returns the ValueSet binding for a given FHIR path from the loaded StructureDefinitions.
func (svc *StructureDefinitionService) GetValuesetBindingForPath(path string) (string, error) {
	for _, structureDefinition := range svc.repo.structureDefinitionsMap {
		for _, element := range structureDefinition.Snapshot.Element {
			if element.Path == path && element.Binding != nil {
				return *element.Binding.ValueSet, nil
			}
		}
	}
	return "", fmt.Errorf("ValueSet binding not found for path: %s", path)
}
