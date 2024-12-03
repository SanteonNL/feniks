package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
)

type CSVMapping struct {
	SourceSystem  string
	SourceCode    string
	SourceDisplay string
	TargetSystem  string
	TargetCode    string
	TargetDisplay string
	IsValid       bool
	ValueSetURI   string
}

type ConceptMapConverter struct {
	log   zerolog.Logger
	cache *ValueSetCache
}

func NewConceptMapConverter(cache *ValueSetCache, log zerolog.Logger) *ConceptMapConverter {
	return &ConceptMapConverter{
		log:   log,
		cache: cache,
	}
}
func (c *ConceptMapConverter) ConvertToFHIR(inputCSV string, outputPath string) error {
	// Read and parse CSV
	mappings, targetValueSet, err := c.readCSV(inputCSV)
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	// Create ConceptMap
	conceptMap := c.createConceptMap(mappings, filepath.Base(inputCSV), targetValueSet)

	// Write to file
	if err := c.writeConceptMap(conceptMap, outputPath); err != nil {
		return fmt.Errorf("failed to write ConceptMap: %w", err)
	}

	return nil
}

func (c *ConceptMapConverter) readCSV(filepath string) ([]CSVMapping, string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	reader.TrimLeadingSpace = true

	// Read headers
	headers, err := reader.Read()
	if err != nil {
		return nil, "", err
	}

	// Find column indices
	indices := getColumnIndices(headers)

	var mappings []CSVMapping
	var targetValueSet string

	for {
		record, err := reader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, "", err
		}

		// Keep track of the ValueSet URI
		if targetValueSet == "" && record[indices.valueSetURI] != "" {
			targetValueSet = record[indices.valueSetURI]
		}

		// Only include valid mappings
		isValid := strings.ToLower(record[indices.targetValidation]) == "valid"
		if isValid || record[indices.targetValidation] == "" { // Include empty validation status
			mapping := CSVMapping{
				SourceSystem:  record[indices.sourceSystem],
				SourceCode:    record[indices.sourceCode],
				SourceDisplay: record[indices.sourceDisplay],
				TargetSystem:  record[indices.targetSystem],
				TargetCode:    record[indices.targetCode],
				TargetDisplay: record[indices.targetDisplay],
				IsValid:       isValid,
				ValueSetURI:   record[indices.valueSetURI],
			}
			mappings = append(mappings, mapping)
		}
	}

	return mappings, targetValueSet, nil
}

func (c *ConceptMapConverter) createConceptMap(mappings []CSVMapping, sourceFile string, targetValueSet string) *fhir.ConceptMap {
	conceptMap := &fhir.ConceptMap{
		//ResourceType: "ConceptMap",
		Status: 1,
		Name:   ptr(fmt.Sprintf("ConceptMap_%s", time.Now().Format("20060102"))),
		//Date:         &fhir.DateTime{Time: time.Now()},
		Description: ptr(fmt.Sprintf("Converted from %s on %s",
			sourceFile, time.Now().Format("2006-01-02 15:04:05"))),
		TargetUri: ptr(targetValueSet),
		Group:     []fhir.ConceptMapGroup{},
	}

	// Group mappings by source system and target system combination
	groupMap := make(map[string]*fhir.ConceptMapGroup)

	for _, mapping := range mappings {
		groupKey := fmt.Sprintf("%s_%s", mapping.SourceSystem, mapping.TargetSystem)
		group, exists := groupMap[groupKey]
		if !exists {
			group = &fhir.ConceptMapGroup{
				Source:  ptr(mapping.SourceSystem),
				Target:  ptr(mapping.TargetSystem),
				Element: []fhir.ConceptMapGroupElement{},
			}
			groupMap[groupKey] = group
		}

		element := fhir.ConceptMapGroupElement{
			Code:    ptr(mapping.SourceCode),
			Display: ptr(mapping.SourceDisplay),
			Target: []fhir.ConceptMapGroupElementTarget{
				{
					Code:        ptr(mapping.TargetCode),
					Display:     ptr(mapping.TargetDisplay),
					Equivalence: 2,
				},
			},
		}

		group.Element = append(group.Element, element)
	}

	// Convert map to slice
	for _, group := range groupMap {
		conceptMap.Group = append(conceptMap.Group, *group)
	}

	return conceptMap
}

type columnIndices struct {
	sourceSystem     int
	sourceCode       int
	sourceDisplay    int
	targetSystem     int
	targetCode       int
	targetDisplay    int
	targetValidation int
	valueSetURI      int
}

func (c *ConceptMapConverter) writeConceptMap(conceptMap *fhir.ConceptMap, outputPath string) error {
	data, err := json.MarshalIndent(conceptMap, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}

func getColumnIndices(headers []string) columnIndices {
	findColumn := func(name string) int {
		for i, h := range headers {
			if strings.EqualFold(strings.TrimSpace(h), name) {
				return i
			}
		}
		return -1
	}

	return columnIndices{
		sourceSystem:     findColumn("system_source"),
		sourceCode:       findColumn("code_source"),
		sourceDisplay:    findColumn("display_source"),
		targetSystem:     findColumn("system_target"),
		targetCode:       findColumn("code_target"),
		targetDisplay:    findColumn("display_target"),
		targetValidation: findColumn("target_validation"),
		valueSetURI:      findColumn("target_valueset_uri"),
	}
}
