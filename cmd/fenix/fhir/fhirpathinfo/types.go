package fhirpathinfo

import (
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/searchparameter"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/structuredefinition"
	"github.com/rs/zerolog"
)

type PathInfo struct {
	Path        string
	ValueSet    string
	SearchTypes map[string]string // code -> search type
}

type PathInfoService struct {
	structDefService   *structuredefinition.StructureDefinitionService
	searchParamService *searchparameter.SearchParameterService
	pathIndex          map[string]*PathInfo
	log                zerolog.Logger
}
