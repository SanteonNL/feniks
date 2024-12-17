package fhirpathinfo

import (
	"fmt"

	"github.com/SanteonNL/fenix/cmd/fenix/fhir/conceptmap"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/searchparameter"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/structuredefinition"
	"github.com/rs/zerolog"
)

// NewPathInfoService creates a new PathInfoService
func NewPathInfoService(
	structDefService *structuredefinition.StructureDefinitionService,
	searchParamService *searchparameter.SearchParameterService,
	conceptMapService *conceptmap.ConceptMapService,
	log zerolog.Logger,
) *PathInfoService {
	return &PathInfoService{
		structDefService:   structDefService,
		searchParamService: searchParamService,
		conceptMapService:  conceptMapService,
		pathIndex:          make(map[string]*PathInfo),
		log:                log,
	}
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

// BuildIndex creates the unified path index
func (svc *PathInfoService) BuildIndex() error {
	svc.pathIndex = make(map[string]*PathInfo)

	// Process structure definitions for ValueSet bindings
	structDefs := svc.structDefService.GetAllStructureDefinitions()
	for _, sd := range structDefs {
		// First get ValueSet bindings from StructureDefinition service
		bindings := svc.structDefService.GetAllPathBindings(sd)
		for path, valueSetURL := range bindings {
			info := svc.getOrCreatePathInfo(path)
			info.ValueSet = valueSetURL

			// Then find all ConceptMaps that reference this ValueSet
			conceptMaps, err := svc.conceptMapService.GetConceptMapsByValuesetURL(valueSetURL)
			if err != nil {
				svc.log.Warn().
					Err(err).
					Str("path", path).
					Str("valueSet", valueSetURL).
					Msg("Failed to get ConceptMaps for ValueSet")
				continue
			}
			info.ConceptMaps = conceptMaps
		}
	}

	// First build the search parameter index
	err := svc.searchParamService.BuildSearchParameterIndex()
	if err != nil {
		return fmt.Errorf("failed to build search parameter index: %w", err)
	}
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

// GetConceptMaps returns all ConceptMaps that reference this field's ValueSet
func (svc *PathInfoService) GetConceptMaps(path string) ([]string, error) {
	info, err := svc.GetPathInfo(path)
	if err != nil {
		return nil, err
	}

	if len(info.ConceptMaps) == 0 {
		return nil, fmt.Errorf("no ConceptMaps found referencing ValueSet %s for path: %s", info.ValueSet, path)
	}

	return info.ConceptMaps, nil
}

// GetSearchTypeByPathAndCode delegates to the SearchParameterService to get the search type
func (svc *PathInfoService) GetSearchTypeByPathAndCode(path string, code string) (string, error) {
	return svc.searchParamService.GetSearchTypeByPathAndCode(path, code)
}
