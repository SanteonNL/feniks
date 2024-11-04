package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

type DataSource struct {
	db           *sqlx.DB
	query        string
	resourceType string
	log          zerolog.Logger
}

type ResourceResult map[string][]RowData

type RowData struct {
	ID       string
	ParentID string
	Data     map[string]interface{}
}

func NewSQLDataSource(db *sqlx.DB, query string, resourceType string, log zerolog.Logger) *DataSource {
	return &DataSource{
		db:           db,
		query:        query,
		resourceType: resourceType,
		log:          log,
	}
}

func (ds *DataSource) ReadResources(patientID string) ([]ResourceResult, error) {
	// Replace parameter in query using resource type
	query := strings.ReplaceAll(ds.query, fmt.Sprintf(":%s.id", ds.resourceType),
		fmt.Sprintf("'%s'", patientID))

	rows, err := ds.db.Queryx(query)
	if err != nil {
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer rows.Close()

	// Map to hold resources by their resource_id
	resources := make(map[string]ResourceResult)

	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		// Remove NULL values
		for key, value := range row {
			if value == nil {
				delete(row, key)
			}
		}

		ds.processRow(row, resources)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	// Convert map to slice
	results := make([]ResourceResult, 0, len(resources))
	for _, result := range resources {
		results = append(results, result)
	}

	return results, nil
}

func (ds *DataSource) processRow(row map[string]interface{}, resources map[string]ResourceResult) {
	id, _ := row["id"].(string)
	parentID, _ := row["parent_id"].(string)
	fhirPath, _ := row["fhir_path"].(string)
	resourceID, _ := row["resource_id"].(string)

	// Initialize resource result if needed
	if resources[resourceID] == nil {
		resources[resourceID] = make(ResourceResult)
	}

	// Process top-level fields
	topLevelData := make(map[string]interface{})
	for key, value := range row {
		// Skip metadata fields
		if key == "id" || key == "parent_id" || key == "fhir_path" || key == "resource_id" {
			continue
		}

		if !strings.Contains(key, ".") {
			topLevelData[key] = value
		} else {
			// Process nested fields
			parts := strings.Split(key, ".")
			ds.processNestedField(parts, value, id, parentID, fhirPath, resourceID, resources)
		}
	}

	// Add top-level fields if any exist
	if len(topLevelData) > 0 {
		if resources[resourceID][fhirPath] == nil {
			resources[resourceID][fhirPath] = []RowData{}
		}

		// Check for existing entry
		existingIndex := -1
		for idx, row := range resources[resourceID][fhirPath] {
			if row.ID == id {
				existingIndex = idx
				break
			}
		}

		if existingIndex != -1 {
			// Update existing entry
			for k, v := range topLevelData {
				resources[resourceID][fhirPath][existingIndex].Data[k] = v
			}
		} else {
			// Add new entry
			resources[resourceID][fhirPath] = append(resources[resourceID][fhirPath], RowData{
				ID:       "",
				ParentID: parentID,
				Data:     topLevelData,
			})
		}
	}
}

func (ds *DataSource) processNestedField(parts []string, value interface{}, id, parentID, fhirPath, resourceID string, resources map[string]ResourceResult) {
	currentPath := fhirPath
	currentID := id
	currentParentID := parentID

	// Build the full path and handle IDs for each level
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		index := ds.extractIndex(part)
		cleanPart := ds.removeIndex(part)

		currentPath += "." + cleanPart

		// For arrays, create a unique ID using the base ID and all indices up to this point
		if index != 0 {
			// For category[1], the ID should be "2"
			// For category[1].coding[0], the ID should be "2_1"
			if i == 0 {
				// First level array (e.g., category[1]) - use index + 1 as ID
				currentID = fmt.Sprintf("%d", index+1)
			} else {
				// Nested array (e.g., coding[0]) - append to parent ID
				currentID = fmt.Sprintf("%s_%d", currentParentID, index+1)
			}
		} else {
			// For index 0
			if i == 0 {
				currentID = "1"
			} else {
				currentID = fmt.Sprintf("%s_1", currentParentID)
			}
		}

		isLeaf := i >= len(parts)-2

		// Ensure path exists in resource result
		if resources[resourceID][currentPath] == nil {
			resources[resourceID][currentPath] = []RowData{}
		}

		// Find existing entry
		existingIndex := -1
		for idx, row := range resources[resourceID][currentPath] {
			if row.ID == currentID {
				existingIndex = idx
				break
			}
		}

		if isLeaf {
			// Process leaf node
			leafField := parts[len(parts)-1]
			if existingIndex != -1 {
				// Update existing entry
				resources[resourceID][currentPath][existingIndex].Data[leafField] = value
			} else {
				// Create new entry
				resources[resourceID][currentPath] = append(resources[resourceID][currentPath], RowData{
					ID:       currentID,
					ParentID: currentParentID,
					Data:     map[string]interface{}{leafField: value},
				})
			}
			break
		} else {
			// Process intermediate node
			if existingIndex == -1 {
				// Create new intermediate node
				resources[resourceID][currentPath] = append(resources[resourceID][currentPath], RowData{
					ID:       currentID,
					ParentID: currentParentID,
					Data:     make(map[string]interface{}),
				})
			}
		}

		// Update parent ID for next iteration
		currentParentID = currentID
	}
}

// Helper function to convert string index to int
func (ds *DataSource) extractIndex(part string) int {
	start := strings.Index(part, "[")
	end := strings.Index(part, "]")
	if start != -1 && end != -1 && start < end {
		index, err := strconv.Atoi(part[start+1 : end])
		if err == nil {
			return index
		}
	}
	return 0
}

func (ds *DataSource) removeIndex(part string) string {
	index := strings.Index(part, "[")
	if index != -1 {
		return part[:index]
	}
	return part
}

// FHIRResourceMap maps resource types to their factory functions
var FHIRResourceMap = map[string]func() interface{}{
	"Patient":       func() interface{} { return &fhir.Patient{} },
	"Observation":   func() interface{} { return &fhir.Observation{} },
	"Encounter":     func() interface{} { return &fhir.Encounter{} },
	"Organization":  func() interface{} { return &fhir.Organization{} },
	"Questionnaire": func() interface{} { return &fhir.Questionnaire{} },
	// Add other resource types as needed
}

// createResource creates a new instance of a FHIR resource
func CreateResource(resourceType string) (interface{}, error) {
	// Get the factory function for the specified resource type
	factory, exists := FHIRResourceMap[resourceType]
	if !exists {
		return nil, fmt.Errorf("unsupported FHIR resource type: %s", resourceType)
	}

	// Create new instance
	resource := factory()
	if resource == nil {
		return nil, fmt.Errorf("failed to create resource of type: %s", resourceType)
	}

	return resource, nil
}
