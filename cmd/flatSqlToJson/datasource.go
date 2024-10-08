package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

type RowData struct {
	ID       string
	ParentID string
	Data     map[string]interface{}
}

type ResourceResult map[string][]RowData

type DataSource interface {
	Read(string) (ResourceResult, error)
	ReadPerPatient(string) ([]ResourceResult, error)
}

type SQLDataSource struct {
	db    *sqlx.DB
	query string
	log   zerolog.Logger
}

func (s *SQLDataSource) processRows(rows *sqlx.Rows) (ResourceResult, error) {
	result := make(ResourceResult)

	for rows.Next() {
		row := make(map[string]interface{})
		err := rows.MapScan(row)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		// Remove NULL values
		for key, value := range row {
			if value == nil {
				delete(row, key)
			}
		}

		id, _ := row["id"].(string)
		parentID, _ := row["parent_id"].(string)
		fhirPath, _ := row["fhir_path"].(string)
		s.log.Debug().Str("id", id).Str("parentID", parentID).Str("fhirPath", fhirPath).Msg("Found row data")

		delete(row, "id")
		delete(row, "parent_id")
		delete(row, "fhir_path")

		mainRow := RowData{
			ID:       id,
			ParentID: parentID,
			Data:     make(map[string]interface{}),
		}

		nestedFields := make(map[string]map[string]interface{})

		for key, value := range row {
			parts := strings.Split(key, ".")
			if len(parts) > 1 {
				nestedFHIRPath := fhirPath + "." + parts[0]
				s.log.Debug().Str("nestedFHIRPath", nestedFHIRPath).Msg("Found nested field")
				if nestedFields[nestedFHIRPath] == nil {
					nestedFields[nestedFHIRPath] = make(map[string]interface{})
				}
				nestedFields[nestedFHIRPath][parts[1]] = value
			} else {
				s.log.Debug().Str("key", key).Msg("Found main field")
				mainRow.Data[key] = value
			}
		}

		result[fhirPath] = append(result[fhirPath], mainRow)

		for nestedPath, nestedData := range nestedFields {
			nestedRow := RowData{
				ID:       id,
				ParentID: id,
				Data:     nestedData,
			}
			result[nestedPath] = append(result[nestedPath], nestedRow)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return result, nil
}

func (s *SQLDataSource) Read(patientID string) (ResourceResult, error) {
	rows, err := s.db.Queryx(s.query)
	if err != nil {
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer rows.Close()

	result := make(ResourceResult)

	for {
		partialResult, err := s.processRows(rows)
		if err != nil {
			return nil, err
		}

		// Merge partialResult into result
		for fhirPath, rowData := range partialResult {
			result[fhirPath] = append(result[fhirPath], rowData...)
		}

		// Move to the next result set
		if !rows.NextResultSet() {
			break // No more result sets
		}
	}

	return result, nil
}

func (s *SQLDataSource) ReadPerPatient(patientID string) ([]ResourceResult, error) {
	query := strings.ReplaceAll(s.query, ":Patient.id", fmt.Sprintf("'%s'", patientID))

	rows, err := s.db.Queryx(query)
	if err != nil {
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer rows.Close()

	resources := make(map[string]ResourceResult)

	for {
		for rows.Next() {
			row := make(map[string]interface{})
			err := rows.MapScan(row)
			if err != nil {
				return nil, fmt.Errorf("error scanning row: %w", err)
			}

			// Remove NULL values
			for key, value := range row {
				if value == nil {
					delete(row, key)
				}
			}

			id, _ := row["id"].(string)
			parentID, _ := row["parent_id"].(string)
			fhirPath, _ := row["fhir_path"].(string)
			resourceID, _ := row["resource_id"].(string)
			s.log.Debug().Str("id", id).Str("parentID", parentID).Str("fhirPath", fhirPath).Str("resourceID", resourceID).Msg("Found row data")

			delete(row, "parent_id")
			delete(row, "fhir_path")
			delete(row, "resource_id")

			mainRow := RowData{
				ID:       id,
				ParentID: parentID,
				Data:     make(map[string]interface{}),
			}

			nestedFields := make(map[string]map[string]interface{})

			for key, value := range row {
				parts := strings.Split(key, ".")
				if len(parts) > 1 {
					nestedFHIRPath := fhirPath + "." + parts[0]
					s.log.Debug().Str("nestedFHIRPath", nestedFHIRPath).Msg("Found nested field")
					if nestedFields[nestedFHIRPath] == nil {
						nestedFields[nestedFHIRPath] = make(map[string]interface{})
					}
					nestedFields[nestedFHIRPath][parts[1]] = value
				} else {
					s.log.Debug().Str("key", key).Msg("Found main field")
					mainRow.Data[key] = value
				}
			}

			if resources[resourceID] == nil {
				resources[resourceID] = make(ResourceResult)
			}
			resources[resourceID][fhirPath] = append(resources[resourceID][fhirPath], mainRow)

			for nestedPath, nestedData := range nestedFields {
				nestedRow := RowData{
					ID:       id,
					ParentID: id,
					Data:     nestedData,
				}
				resources[resourceID][nestedPath] = append(resources[resourceID][nestedPath], nestedRow)
			}
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating over rows: %w", err)
		}

		if !rows.NextResultSet() {
			break // No more result sets
		}
	}

	// Convert map of ResourceResults to slice of ResourceResults
	//TODO  Make sure to return map instead of converting to a slice
	results := make([]ResourceResult, 0, len(resources))
	for _, result := range resources {
		results = append(results, result)
	}

	return results, nil
}

func NewSQLDataSource(db *sqlx.DB, query string, log zerolog.Logger) *SQLDataSource {
	return &SQLDataSource{
		db:    db,
		query: query,
		log:   log,
	}
}

type CSVDataSource struct {
	filePath string
	mapper   *CSVMapper
}

type CSVMapper struct {
	Mappings []CSVMapping
}

type CSVMapping struct {
	FieldName string
	Files     []CSVFileMapping
}

type CSVFileMapping struct {
	FileName      string
	FieldMappings []CSVFieldMapping
}

type CSVFieldMapping struct {
	FHIRField     map[string]string
	ParentIDField string
	IDField       string
}

type CSVMapperConfig struct {
	Mappings []struct {
		FieldName string `json:"fieldName"`
		Files     []struct {
			FileName      string `json:"fileName"`
			FieldMappings []struct {
				CSVFields     map[string]string `json:"csvFields"`
				IDField       string            `json:"idField"`
				ParentIDField string            `json:"parentIdField"`
			} `json:"fieldMappings"`
		} `json:"files"`
	} `json:"mappings"`
}

func LoadCSVMapperFromConfig(filePath string) (*CSVMapper, error) {
	jsonFile, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config CSVMapperConfig
	err = json.Unmarshal(jsonFile, &config)
	if err != nil {
		return nil, err
	}

	mapper := &CSVMapper{
		Mappings: make([]CSVMapping, len(config.Mappings)),
	}

	for i, configMapping := range config.Mappings {
		mapping := CSVMapping{
			FieldName: configMapping.FieldName,
			Files:     make([]CSVFileMapping, len(configMapping.Files)),
		}

		for j, configFile := range configMapping.Files {
			fileMapping := CSVFileMapping{
				FileName:      configFile.FileName,
				FieldMappings: make([]CSVFieldMapping, len(configFile.FieldMappings)),
			}

			for k, configFieldMapping := range configFile.FieldMappings {
				fieldMapping := CSVFieldMapping{
					FHIRField:     configFieldMapping.CSVFields,
					IDField:       configFieldMapping.IDField,
					ParentIDField: configFieldMapping.ParentIDField,
				}
				fileMapping.FieldMappings[k] = fieldMapping
			}

			mapping.Files[j] = fileMapping
		}

		mapper.Mappings[i] = mapping
	}

	return mapper, nil
}

func NewCSVDataSource(filePath string, mapper *CSVMapper) *CSVDataSource {
	return &CSVDataSource{
		filePath: filePath,
		mapper:   mapper,
	}
}

func (c *CSVDataSource) Read() (map[string][]map[string]interface{}, error) {
	result := make(map[string][]map[string]interface{})

	for _, mapping := range c.mapper.Mappings {
		for _, fileMapping := range mapping.Files {
			fileData, err := c.readFile(fileMapping)
			if err != nil {
				return nil, err
			}
			result[mapping.FieldName] = append(result[mapping.FieldName], fileData...)
		}
	}

	return result, nil
}

func (c *CSVDataSource) readFile(fileMapping CSVFileMapping) ([]map[string]interface{}, error) {
	file, err := os.Open(fileMapping.FileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, header := range headers {
			for _, fieldMapping := range fileMapping.FieldMappings {
				if fhirField, ok := fieldMapping.FHIRField[header]; ok {
					if record[i] != "" {
						row[fhirField] = record[i]
					}
				}
				if header == fieldMapping.ParentIDField {
					row["parent_id"] = record[i]
				}
				if header == fieldMapping.IDField {
					row["id"] = record[i]
				}
			}
		}

		result = append(result, row)
	}

	return result, nil
}
