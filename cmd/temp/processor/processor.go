package processor

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/SanteonNL/fenix/cmd/fenix/fhir/conceptmap"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/structuredefinition"
	"github.com/SanteonNL/fenix/cmd/fenix/fhir/valueset"
	"github.com/SanteonNL/fenix/models/fhir"

	"github.com/rs/zerolog"
)

// DataSource interface defines what we expect from a data source
type DataSource interface {
	ReadResources(patientID string) ([]ResourceResult, error)
}

// ResourceResult represents the data for a single resource
type ResourceResult map[string][]RowData

// RowData represents a single row of data
type RowData struct {
	ID       string
	ParentID string
	Data     map[string]interface{}
}

// ProcessorService handles processing of FHIR resources from a data source
type ProcessorService struct {
	log                        zerolog.Logger
	valueSetService            *valueset.ValueSetRepository
	conceptMapService          *conceptmap.ConceptMapService
	structureDefinitionService *structuredefinition.StructureDefinitionService
	processedPaths             sync.Map
	searchParams               map[string]SearchParameter
}

// SearchParameter defines filtering criteria
type SearchParameter struct {
	Code       string
	Type       string
	Modifier   []string
	Comparator string
	Value      string
}

// FilterResult represents the outcome of a filter operation
type FilterResult struct {
	Passed  bool
	Message string
}

// NewProcessorService creates a new processor service instance
func NewProcessorService(
	log zerolog.Logger,
	valueSetCache *valueset.ValueSetRepository,
	conceptMapSvc *conceptmap.ConceptMapService,
	structDefSvc *structuredefinition.StructureDefinitionService,
	searchParams map[string]SearchParameter,
) *ProcessorService {
	return &ProcessorService{
		log:           log,
		valueSetCache: valueSetCache,
		conceptMapSvc: conceptMapSvc,
		structDefSvc:  structDefSvc,
		searchParams:  searchParams,
	}
}

// ProcessResources processes resources from a data source
func (p *ProcessorService) ProcessResources(ctx context.Context, ds DataSource, patientID string) ([]interface{}, error) {
	// Read resources from data source
	results, err := ds.ReadResources(patientID)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	p.log.Info().Msgf("Number of results found: %d", len(results))

	var processedResources []interface{}
	for _, result := range results {
		// Process each resource result
		resource, err := p.processResourceResult(ctx, result)
		if err != nil {
			p.log.Error().Err(err).Msg("Error processing resource")
			continue
		}

		if resource != nil {
			processedResources = append(processedResources, resource)
		}
	}

	return processedResources, nil
}

// processResourceResult processes a single resource result
func (p *ProcessorService) processResourceResult(ctx context.Context, result ResourceResult) (interface{}, error) {
	// Initialize the appropriate resource type
	resource, err := p.createResource(result)
	if err != nil {
		return nil, fmt.Errorf("error creating resource: %w", err)
	}

	resourceValue := reflect.ValueOf(resource).Elem()
	filterResult, err := p.populateResourceStruct(ctx, resourceValue, result)
	if err != nil {
		return nil, fmt.Errorf("error populating resource: %w", err)
	}

	if !filterResult.Passed {
		p.log.Debug().Msg("Resource filtered out")
		return nil, nil
	}

	return resource, nil
}

// populateResourceStruct populates a resource structure with data
func (p *ProcessorService) populateResourceStruct(ctx context.Context, value reflect.Value, result ResourceResult) (*FilterResult, error) {
	return p.determinePopulateType(ctx, "", value, "", result)
}

// determinePopulateType handles different field types during population
// Helper method to set basic type fields
func (p *ProcessorService) setBasicType(ctx context.Context, path string, field reflect.Value, parentID string, rows []RowData) (*FilterResult, error) {
	for _, row := range rows {
		if row.ParentID == parentID || parentID == "" {
			for key, value := range row.Data {
				if err := p.setField(ctx, path, field.Addr().Interface(), key, value); err != nil {
					return nil, err
				}

				// Check filter
				filterResult, err := p.checkFilter(ctx, path, field)
				if err != nil {
					return nil, err
				}
				return filterResult, nil
			}
		}
	}
	return &FilterResult{Passed: true}, nil
}

// populateStructAndNestedFields handles both struct fields and nested fields
func (p *ProcessorService) populateStructAndNestedFields(ctx context.Context, structPath string, value reflect.Value, row RowData, result ResourceResult) (*FilterResult, error) {
	// First populate struct fields
	structResult, err := p.populateStructFields(ctx, structPath, value.Addr().Interface(), row, result)
	if err != nil {
		return nil, fmt.Errorf("failed to populate struct fields at %s: %w", structPath, err)
	}

	if !structResult.Passed {
		return structResult, nil
	}

	// Then handle nested fields
	nestedResult, err := p.populateNestedFields(ctx, structPath, value, result, row.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to populate nested fields at %s: %w", structPath, err)
	}

	return nestedResult, nil
}

// populateNestedFields handles nested field population
func (p *ProcessorService) populateNestedFields(ctx context.Context, parentPath string, parentValue reflect.Value, result ResourceResult, parentID string) (*FilterResult, error) {
	for i := 0; i < parentValue.NumField(); i++ {
		field := parentValue.Field(i)
		fieldName := parentValue.Type().Field(i).Name
		fieldPath := fmt.Sprintf("%s.%s", parentPath, strings.ToLower(fieldName))

		// Skip already processed paths
		if _, processed := p.processedPaths.Load(fieldPath); processed {
			continue
		}

		if hasDataForPath(result, fieldPath) {
			p.processedPaths.Store(fieldPath, true)
			filterResult, err := p.determinePopulateType(ctx, fieldPath, field, parentID, result)
			if err != nil {
				return nil, err
			}
			if !filterResult.Passed {
				return filterResult, nil
			}
		}
	}

	return &FilterResult{Passed: true}, nil
}

func (p *ProcessorService) determinePopulateType(ctx context.Context, structPath string, value reflect.Value, parentID string, result ResourceResult) (*FilterResult, error) {
	p.log.Debug().
		Str("structPath", structPath).
		Str("valueKind", value.Kind().String()).
		Msg("Determining populate type")

	rows, exists := result[structPath]
	if !exists {
		return &FilterResult{Passed: true}, nil
	}

	switch value.Kind() {
	case reflect.Slice:
		return p.populateSlice(ctx, structPath, value, parentID, rows, result)
	case reflect.Struct:
		return p.populateStruct(ctx, structPath, value, parentID, rows, result)
	case reflect.Ptr:
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return p.determinePopulateType(ctx, structPath, value.Elem(), parentID, result)
	default:
		return p.setBasicType(ctx, structPath, value, parentID, rows)
	}
}

// populateSlice handles populating slice fields
func (p *ProcessorService) populateSlice(ctx context.Context, structPath string, value reflect.Value, parentID string, rows []RowData, result ResourceResult) (*FilterResult, error) {
	p.log.Debug().
		Str("structPath", structPath).
		Msg("Populating slice")

	allElements := reflect.MakeSlice(value.Type(), 0, len(rows))
	anyElementPassed := false

	hasFilter := false
	if _, exists := p.searchParams[structPath]; exists {
		hasFilter = true
	}

	for _, row := range rows {
		if row.ParentID == parentID || row.ParentID == "" {
			valueElement := reflect.New(value.Type().Elem()).Elem()

			filterResult, err := p.populateStructAndNestedFields(ctx, structPath, valueElement, row, result)
			if err != nil {
				return nil, fmt.Errorf("error populating slice element: %w", err)
			}

			if hasFilter && filterResult.Passed {
				elementFilterResult, err := p.checkFilter(ctx, structPath, valueElement)
				if err != nil {
					return nil, fmt.Errorf("error checking filter for slice element: %w", err)
				}
				if elementFilterResult.Passed {
					anyElementPassed = true
				}
			} else if !hasFilter {
				anyElementPassed = true
			}

			allElements = reflect.Append(allElements, valueElement)
		}
	}

	if hasFilter && !anyElementPassed {
		return &FilterResult{
			Passed:  false,
			Message: fmt.Sprintf("No elements in slice at %s passed filters", structPath),
		}, nil
	}

	value.Set(allElements)
	return &FilterResult{Passed: true}, nil
}

// populateStruct handles populating struct fields
func (p *ProcessorService) populateStruct(ctx context.Context, path string, value reflect.Value, parentID string, rows []RowData, result ResourceResult) (*FilterResult, error) {
	if filterResult, err := p.checkFilter(ctx, path, value); err != nil {
		return nil, fmt.Errorf("failed to check struct filter at %s: %w", path, err)
	} else if !filterResult.Passed {
		return filterResult, nil
	}

	anyFieldsPopulated := false
	var lastFilterResult *FilterResult

	for _, row := range rows {
		if row.ParentID == parentID || parentID == "" {
			// Populate direct fields
			filterResult, err := p.populateStructFields(ctx, path, value.Addr().Interface(), row, result)
			if err != nil {
				return nil, fmt.Errorf("failed to populate struct fields at %s: %w", path, err)
			}

			if !filterResult.Passed {
				return filterResult, nil
			}

			// Handle nested fields
			nestedResult, err := p.populateNestedFields(ctx, path, value, result, row.ID)
			if err != nil {
				return nil, err
			}

			if !nestedResult.Passed {
				return nestedResult, nil
			}

			anyFieldsPopulated = true
			lastFilterResult = nestedResult
		}
	}

	if !anyFieldsPopulated {
		return &FilterResult{Passed: true}, nil
	}

	return lastFilterResult, nil
}

// populateStructFields handles populating the fields of a struct
func (p *ProcessorService) populateStructFields(ctx context.Context, structPath string, structPtr interface{}, row RowData, result ResourceResult) (*FilterResult, error) {
	structValue := reflect.ValueOf(structPtr).Elem()
	structType := structValue.Type()
	processedFields := make(map[string]bool)

	// First process all Coding and CodeableConcept fields
	for i := 0; i < structType.NumField(); i++ {
		field := structValue.Field(i)
		fieldType := field.Type().String()
		fieldName := structType.Field(i).Name

		if strings.Contains(fieldType, "Coding") || strings.Contains(fieldType, "CodeableConcept") {
			codingPath := fmt.Sprintf("%s.%s", structPath, strings.ToLower(fieldName))
			p.processedPaths.Store(codingPath, true)

			codingRows, exists := result[codingPath]
			if !exists {
				continue
			}

			if strings.Contains(fieldType, "CodeableConcept") {
				if err := p.setCodeableConceptField(ctx, field, codingPath, fieldName, row.ID, codingRows, processedFields); err != nil {
					if _, hasFilter := p.searchParams[codingPath]; hasFilter {
						return &FilterResult{
							Passed:  false,
							Message: fmt.Sprintf("Filter failed for %s: %v", codingPath, err),
						}, nil
					}
					p.log.Debug().Err(err).Str("path", codingPath).Msg("Error processing CodeableConcept field")
					continue
				}
			} else {
				for _, codingRow := range codingRows {
					if codingRow.ParentID == row.ID {
						if err := p.setCodingFromRow(ctx, codingPath, field, fieldName, codingRow, processedFields); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}

	// Then process regular fields
	for key, value := range row.Data {
		if processedFields[key] {
			continue
		}

		for i := 0; i < structType.NumField(); i++ {
			fieldName := structType.Field(i).Name
			if processedFields[fieldName] {
				continue
			}

			if strings.EqualFold(fieldName, key) {
				if err := p.setField(ctx, structPath, structPtr, fieldName, value); err != nil {
					return nil, fmt.Errorf("failed to set field %s: %w", fieldName, err)
				}
				processedFields[fieldName] = true
				break
			}
		}
	}

	return &FilterResult{Passed: true}, nil
}

// setCodeableConceptField handles setting CodeableConcept fields
func (p *ProcessorService) setCodeableConceptField(ctx context.Context, field reflect.Value, path string, fieldName string, parentID string, rows []RowData, processedFields map[string]bool) error {
	p.log.Debug().
		Str("path", path).
		Str("fieldName", fieldName).
		Str("parentID", parentID).
		Msg("Setting CodeableConcept field")

	isSlice := field.Kind() == reflect.Slice

	if isSlice {
		if field.IsNil() {
			field.Set(reflect.MakeSlice(field.Type(), 0, len(rows)))
		}

		conceptRows := make(map[string][]RowData)
		for _, row := range rows {
			if row.ParentID == parentID {
				conceptRows[row.ID] = append(conceptRows[row.ID], row)
			}
		}

		anyElementPassed := false
		hasFilter := false
		if _, exists := p.searchParams[path]; exists {
			hasFilter = true
		}
	}
}

func (p *ProcessorService) checkFilter(ctx context.Context, path string, field reflect.Value) (*FilterResult, error) {
	param, exists := p.searchParams[path]
	if !exists {
		return &FilterResult{Passed: true}, nil
	}

	switch param.Type {
	case "token":
		return p.checkTokenFilter(ctx, field, param)
	case "date":
		return p.checkDateFilter(ctx, field, param)
	default:
		return &FilterResult{Passed: true}, nil
	}
}

// createResource creates a new FHIR resource instance
func (p *ProcessorService) createResource(result ResourceResult) (interface{}, error) {
	// Determine resource type from the data
	var resourceType string
	for path := range result {
		parts := strings.Split(path, ".")
		if len(parts) > 0 {
			resourceType = parts[0]
			break
		}
	}

	factory, exists := fhirResourceMap[resourceType]
	if !exists {
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	return factory(), nil
}

// fhirResourceMap maps resource types to their factory functions
var fhirResourceMap = map[string]func() interface{}{
	"Patient":       func() interface{} { return &fhir.Patient{} },
	"Observation":   func() interface{} { return &fhir.Observation{} },
	"Encounter":     func() interface{} { return &fhir.Encounter{} },
	"Organization":  func() interface{} { return &fhir.Organization{} },
	"Questionnaire": func() interface{} { return &fhir.Questionnaire{} },
}
