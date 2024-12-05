// internal/fhir/conceptmap/converter.go
package conceptmap

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
)

// ConceptMapConverter handles conversion of mapping files to FHIR ConceptMaps
type ConceptMapConverter struct {
	log               zerolog.Logger
	conceptMapService *ConceptMapService
}

// NewConceptMapConverter creates a new converter instance
func NewConceptMapConverter(log zerolog.Logger, conceptMapService *ConceptMapService) *ConceptMapConverter {
	return &ConceptMapConverter{
		log:               log,
		conceptMapService: conceptMapService,
	}
}

// ConvertCSVToFHIR converts a CSV file to a FHIR ConceptMap
func (c *ConceptMapConverter) ConvertCSVToFHIR(reader io.Reader, name string) (*fhir.ConceptMap, error) {
	csvReader := csv.NewReader(reader)
	csvReader.Comma = ';'
	csvReader.TrimLeadingSpace = true

	// Read and validate headers
	headers, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	indices := getColumnIndices(headers)
	if !indices.areValid() {
		return nil, fmt.Errorf("required columns not found in CSV")
	}

	// Create initial ConceptMap
	conceptMap := c.conceptMapService.CreateConceptMap(
		fmt.Sprintf("%s_%s", name, time.Now().Format("20060102")),
		name,
		"", // Will be populated from first row
		"", // Will be populated from first row
	)

	// Track processed systems for efficient grouping
	groupMap := make(map[string]*fhir.ConceptMapGroup)

	// Process each row
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read row: %w", err)
		}

		mapping, err := c.extractMapping(row, indices)
		if err != nil {
			return nil, fmt.Errorf("failed to extract mapping from row: %w", err)
		}

		// Update ValueSet URI from first valid row
		if conceptMap.TargetUri == nil && mapping.ValueSetURI != "" {
			uri := mapping.ValueSetURI
			conceptMap.TargetUri = &uri
		}

		// Add mapping to ConceptMap
		if err := c.addMappingToConceptMap(conceptMap, mapping, groupMap); err != nil {
			return nil, fmt.Errorf("failed to add mapping: %w", err)
		}
	}

	// Convert groupMap to final groups slice
	conceptMap.Group = c.finalizeGroups(groupMap)

	return conceptMap, nil
}

// extractMapping creates a CSVMapping from a CSV row
func (c *ConceptMapConverter) extractMapping(row []string, indices columnIndices) (*CSVMapping, error) {
	if len(row) <= indices.maxIndex() {
		return nil, fmt.Errorf("row has insufficient columns")
	}

	mapping := &CSVMapping{
		SourceSystem:  strings.TrimSpace(row[indices.sourceSystem]),
		SourceCode:    strings.TrimSpace(row[indices.sourceCode]),
		SourceDisplay: strings.TrimSpace(row[indices.sourceDisplay]),
		TargetSystem:  strings.TrimSpace(row[indices.targetSystem]),
		TargetCode:    strings.TrimSpace(row[indices.targetCode]),
		TargetDisplay: strings.TrimSpace(row[indices.targetDisplay]),
	}

	// Validate required fields
	if mapping.SourceCode == "" || mapping.TargetCode == "" {
		return nil, fmt.Errorf("source and target codes are required")
	}

	// Add ValueSet URI if available
	if indices.valueSetURI != -1 && indices.valueSetURI < len(row) {
		mapping.ValueSetURI = strings.TrimSpace(row[indices.valueSetURI])
	}

	return mapping, nil
}

// addMappingToConceptMap adds a mapping to the ConceptMap, handling group organization
func (c *ConceptMapConverter) addMappingToConceptMap(
	conceptMap *fhir.ConceptMap,
	mapping *CSVMapping,
	groupMap map[string]*fhir.ConceptMapGroup,
) error {
	// Create group key from source and target systems
	groupKey := fmt.Sprintf("%s|%s", mapping.SourceSystem, mapping.TargetSystem)

	// Get or create group
	group, exists := groupMap[groupKey]
	if !exists {
		group = &fhir.ConceptMapGroup{
			Source:  &mapping.SourceSystem,
			Target:  &mapping.TargetSystem,
			Element: make([]fhir.ConceptMapGroupElement, 0),
		}
		groupMap[groupKey] = group
	}

	// Create new element
	element := fhir.ConceptMapGroupElement{
		Code:    &mapping.SourceCode,
		Display: &mapping.SourceDisplay,
		Target: []fhir.ConceptMapGroupElementTarget{
			{
				Code:        &mapping.TargetCode,
				Display:     &mapping.TargetDisplay,
				Equivalence: 2, // equivalent by default
			},
		},
	}

	// Add element to group
	group.Element = append(group.Element, element)

	return nil
}

// finalizeGroups converts the group map to a sorted slice of groups
func (c *ConceptMapConverter) finalizeGroups(groupMap map[string]*fhir.ConceptMapGroup) []fhir.ConceptMapGroup {
	groups := make([]fhir.ConceptMapGroup, 0, len(groupMap))
	for _, group := range groupMap {
		groups = append(groups, *group)
	}
	return groups
}
