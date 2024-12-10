package fhirpathinfo

import (
	"fmt"

	"github.com/SanteonNL/fenix/cmd/fenix/fhir/searchparameter"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/structuredefinition"
	"github.com/rs/zerolog"
)

// NewPathInfoService creates a new PathInfoService
func NewPathInfoService(
	structDefService *structuredefinition.StructureDefinitionService,
	searchParamService *searchparameter.SearchParameterService,
	log zerolog.Logger,
) *PathInfoService {
	return &PathInfoService{
		structDefService:   structDefService,
		searchParamService: searchParamService,
		pathIndex:          make(map[string]*PathInfo),
		log:                log,
	}
}

// BuildIndex creates the unified path index
func (svc *PathInfoService) BuildIndex() error {
	svc.pathIndex = make(map[string]*PathInfo)

	// First, process structure definitions for ValueSet bindings
	structDefs := svc.structDefService.GetAllStructureDefinitions()
	for _, sd := range structDefs {
		bindings := svc.structDefService.GetAllPathBindings(sd)
		for path, valueSet := range bindings {
			info := svc.getOrCreatePathInfo(path)
			info.ValueSet = valueSet
		}
	}

	// Then, process search parameters using the service method
	searchTypesMap := svc.searchParamService.GetAllPathSearchTypes()
	for path, codeMap := range searchTypesMap {
		info := svc.getOrCreatePathInfo(path)
		for code, searchType := range codeMap {
			info.SearchTypes[code] = searchType
		}
	}

	// Log summary
	svc.log.Info().
		Int("total_paths", len(svc.pathIndex)).
		Msg("Completed building path index")

	return nil
}

// getOrCreatePathInfo gets or creates a PathInfo for a path
func (svc *PathInfoService) getOrCreatePathInfo(path string) *PathInfo {
	if info, exists := svc.pathIndex[path]; exists {
		return info
	}

	info := &PathInfo{
		Path:        path,
		SearchTypes: make(map[string]string),
	}
	svc.pathIndex[path] = info
	return info
}

// GetPathInfo returns complete information for a path
func (svc *PathInfoService) GetPathInfo(path string) (*PathInfo, error) {
	info, exists := svc.pathIndex[path]
	if !exists {
		return nil, fmt.Errorf("no information found for path: %s", path)
	}
	return info, nil
}

// GetSearchTypeByCode returns the search type for a specific path and code
func (svc *PathInfoService) GetSearchTypeByCode(path, code string) (string, error) {
	info, err := svc.GetPathInfo(path)
	if err != nil {
		return "", err
	}

	searchType, exists := info.SearchTypes[code]
	if !exists {
		return "", fmt.Errorf("no search type found for path %s and code %s", path, code)
	}

	return searchType, nil
}

// GetValueSet returns the ValueSet URL for a path
func (svc *PathInfoService) GetValueSet(path string) (string, error) {
	info, err := svc.GetPathInfo(path)
	if err != nil {
		return "", err
	}

	if info.ValueSet == "" {
		return "", fmt.Errorf("no ValueSet found for path: %s", path)
	}

	return info.ValueSet, nil
}
