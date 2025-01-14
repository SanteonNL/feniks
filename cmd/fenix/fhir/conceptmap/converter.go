// internal/fhir/conceptmap/converter.go
package conceptmap

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/SanteonNL/fenix/util"
	"github.com/rs/zerolog"
)

// ConceptMapConverter handles conversion of mapping files to FHIR ConceptMaps
type ConceptMapConverter struct {
	log           zerolog.Logger
	baseDir       string
	InputDir      string
	RepositoryDir string
}

// NewConceptMapConverter creates a new converter instance
func NewConceptMapConverter(log zerolog.Logger) (*ConceptMapConverter, error) {
	converter := &ConceptMapConverter{
		log:     log,
		baseDir: ".",
	}

	// Setup directories
	if err := converter.SetupDirectories(); err != nil {
		log.Error().Err(err).Msg("Failed to set up directories for ConceptMap conversion")
		return nil, fmt.Errorf("failed to create ConceptMapConverter: %w", err)
	}

	return converter, nil
}

func (c *ConceptMapConverter) SetupDirectories() error {
	// Define paths based on the base directory
	c.InputDir = filepath.Join(c.baseDir, "config/conceptmaps/flat")
	c.RepositoryDir = filepath.Join(c.baseDir, "config/conceptmaps/fhir/converted")

	// Ensure directories exist
	for _, dir := range []string{c.InputDir, c.RepositoryDir} {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			c.log.Error().Err(err).Str("path", dir).Msg("Failed to create directory")
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		c.log.Info().Str("directory", dir).Msg("Directory verified and ready")
	}

	return nil
}

// TODO: add check if csv is already converted
// ConvertFolderToFHIR converts all CSV files in a folder to FHIR ConceptMaps
func (c *ConceptMapConverter) ConvertFolderToFHIR(inputFolder string, repository *ConceptMapRepository, usePrefix bool) error {
	files, err := os.ReadDir(inputFolder)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	var conversionErrors []string
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(strings.ToLower(file.Name()), ".csv") {
			continue
		}

		filePath := filepath.Join(inputFolder, file.Name())
		csvFile, err := os.Open(filePath)
		if err != nil {
			conversionErrors = append(conversionErrors, fmt.Sprintf("failed to open %s: %v", file.Name(), err))
			continue
		}

		err = c.ConvertCSVToFHIRAndSave(csvFile, file.Name(), repository, usePrefix)
		csvFile.Close()
		if err != nil {
			conversionErrors = append(conversionErrors, fmt.Sprintf("failed to convert %s: %v", file.Name(), err))
			continue
		}

		c.log.Info().
			Str("file", file.Name()).
			Msg("Successfully converted CSV to ConceptMap")
	}

	if len(conversionErrors) > 0 {
		return fmt.Errorf("encountered errors during conversion:\n%s", strings.Join(conversionErrors, "\n"))
	}

	return nil
}

// ConvertCSVToFHIRAndSave converts a CSV file to a FHIR ConceptMap and saves it to the repository's converted folder
func (c *ConceptMapConverter) ConvertCSVToFHIRAndSave(reader io.Reader, csvName string, repository *ConceptMapRepository, usePrefix bool) error {
	csvReader := csv.NewReader(reader)
	csvReader.Comma = ';'
	csvReader.TrimLeadingSpace = true

	// Read and validate headers
	headers, err := csvReader.Read()
	if err != nil {
		return fmt.Errorf("failed to read headers: %w", err)
	}

	indices := getColumnIndices(headers)
	if !indices.areValid() {
		return fmt.Errorf("required columns not found in CSV")
	}

	// Remove .csv extension if present
	baseName := strings.TrimSuffix(csvName, filepath.Ext(csvName))

	// Create initial ConceptMap
	conceptMap := c.CreateConceptMap(
		fmt.Sprintf("%s_%s", baseName, time.Now().Format("20060102")),
		baseName,
		"", // Will be populated from first row
		"", // Will be populated from first row
	)

	// Track processed systems for efficient grouping
	groupMap := make(map[string]*fhir.ConceptMapGroup)

	// Process each row
	var firstRowProcessed bool
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read row: %w", err)
		}

		mapping, err := c.extractMapping(row, indices)
		if err != nil {
			return fmt.Errorf("failed to extract mapping from row: %w", err)
		}

		// Update ValueSet URI from first valid row
		if !firstRowProcessed && mapping.ValueSetURI != "" {
			uri := mapping.ValueSetURI
			// if usePrefix {
			// 	// Add prefix to the last segment of the URI
			// 	segments := strings.Split(uri, "/")
			// 	if len(segments) > 0 {
			// 		lastIndex := len(segments) - 1
			// 		segments[lastIndex] = "conceptmap_converted_" + segments[lastIndex]
			// 		uri = strings.Join(segments, "/")
			// 	}
			// }
			conceptMap.TargetUri = &uri
			firstRowProcessed = true
		}

		// Add mapping to ConceptMap
		if err := c.addMappingToConceptMap(conceptMap, mapping, groupMap); err != nil {
			return fmt.Errorf("failed to add mapping: %w", err)
		}
	}

	// Convert groupMap to final groups slice
	conceptMap.Group = c.finalizeGroups(groupMap)

	// Create the fhir/converted directory within the repository's local path
	if err := os.MkdirAll(repository.localPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save with original CSV name (minus extension) plus .json
	outputFile := filepath.Join(repository.localPath, baseName+".json")
	if err := c.SaveConceptMap(outputFile, conceptMap); err != nil {
		return fmt.Errorf("failed to save ConceptMap: %w", err)
	}

	return nil
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

func (c *ConceptMapConverter) CreateConceptMap(id string, name string, sourceValueSet string, targetValueSet string) *fhir.ConceptMap {
	url := fmt.Sprintf("http://localhost/fhir/ConceptMap/%s", id)

	return &fhir.ConceptMap{
		Id:        &id,
		Url:       &url,
		Name:      &name,
		Status:    1,
		Date:      util.StringPtr(time.Now().Format(time.RFC3339)),
		SourceUri: &sourceValueSet,
		TargetUri: &targetValueSet,
		Group:     []fhir.ConceptMapGroup{},
	}
}

// Add this method to your existing conceptmap/service.go file
func (c *ConceptMapConverter) SaveConceptMap(outputPath string, cm *fhir.ConceptMap) error {
	data, err := json.MarshalIndent(cm, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ConceptMap: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		c.log.Error().Err(err).Str("path", outputPath).Msg("Failed to write ConceptMap to file")
		return fmt.Errorf("failed to write ConceptMap file: %w", err)
	}

	c.log.Debug().Str("path", outputPath).Msg("Successfully saved ConceptMap")
	return nil
}
