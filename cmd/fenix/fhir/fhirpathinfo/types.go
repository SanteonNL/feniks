package fhirpathinfo

import (
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/conceptmap"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/searchparameter"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/structuredefinition"
	"github.com/rs/zerolog"
)

type PathInfoService struct {
	structDefService   *structuredefinition.StructureDefinitionService
	searchParamService *searchparameter.SearchParameterService
	conceptMapService  *conceptmap.ConceptMapService
	pathIndex          map[string]*PathInfo
	log                zerolog.Logger
}

// PathInfo represents complete path information
type PathInfo struct {
	Path        string
	ValueSet    string   // ValueSet URL from StructureDefinition
	ConceptMaps []string // All ConceptMaps that reference this ValueSet
	SearchTypes map[string]string
}
