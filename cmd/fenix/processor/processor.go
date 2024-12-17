package processor

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/SanteonNL/fenix/cmd/fenix/datasource"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/conceptmap"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/fhirpathinfo"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/valueset"
	"github.com/SanteonNL/fenix/cmd/fenix/output"
	"github.com/rs/zerolog"
)

type ProcessorService struct {
	log            zerolog.Logger
	pathInfoSvc    *fhirpathinfo.PathInfoService
	valueSetSvc    *valueset.ValueSetService
	conceptMapSvc  *conceptmap.ConceptMapService
	outputManager  *output.OutputManager
	processedPaths sync.Map
}

func NewProcessorService(
	log zerolog.Logger,
	pathInfoSvc *fhirpathinfo.PathInfoService,
	valueSetSvc *valueset.ValueSetService,
	conceptMapSvc *conceptmap.ConceptMapService,
	outputManager *output.OutputManager,
) *ProcessorService {
	return &ProcessorService{
		log:           log,
		pathInfoSvc:   pathInfoSvc,
		valueSetSvc:   valueSetSvc,
		conceptMapSvc: conceptMapSvc,
		outputManager: outputManager,
	}
}

// ProcessResources processes resources with filtering
func (p *ProcessorService) ProcessResources(ctx context.Context, ds *datasource.DataSourceService, patientID string, filter *Filter) ([]interface{}, error) {
	results, err := ds.ReadResources("Patient", patientID)
	if err != nil {
		return nil, fmt.Errorf("failed to read resources: %w", err)
	}

	err = p.outputManager.WriteToJSON(results, "temp_result")
	if err != nil {
		return nil, fmt.Errorf("failed to write resources to JSON: %w", err)
	}

	var processedResources []interface{}
	for _, result := range results {
		resource, err := p.createResource(result)
		if err != nil {
			p.log.Error().Err(err).Msg("Error creating resource")
			continue
		}

		p.log.Info().Msgf("Processing resource: %v", result)

		passed, err := p.populateAndFilter(ctx, resource, result, filter)
		if err != nil {
			p.log.Error().Err(err).Msg("Error processing resource")
			continue
		}

		if passed {
			processedResources = append(processedResources, resource)
		}
	}

	return processedResources, nil
}

// createResource creates a new instance of the appropriate resource type
func (p *ProcessorService) createResource(result datasource.ResourceResult) (interface{}, error) {
	var resourceType string
	for path := range result {
		parts := strings.Split(path, ".")
		if len(parts) > 0 {
			resourceType = parts[0]
			break
		}
	}

	factory, exists := ResourceFactoryMap[resourceType]
	if !exists {
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	return factory(), nil
}
