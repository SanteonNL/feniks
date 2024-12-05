package conceptmap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
)

// ConceptMapRepository handles loading and storing ConceptMap resources.
type ConceptMapRepository struct {
	log         zerolog.Logger
	localPath   string
	cache       sync.Map
	conceptMaps map[string]fhir.ConceptMap
}

// NewConceptMapRepository creates a new ConceptMapRepository.
func NewConceptMapRepository(log zerolog.Logger, localPath string) *ConceptMapRepository {
	return &ConceptMapRepository{
		log:         log,
		localPath:   localPath,
		conceptMaps: make(map[string]fhir.ConceptMap),
	}
}

// LoadConceptMaps loads all ConceptMaps into the repository.
func (repo *ConceptMapRepository) LoadConceptMaps() error {
	files, err := os.ReadDir(repo.localPath)
	if err != nil {
		repo.log.Error().Err(err).Msg("Failed to read directory")
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			filePath := filepath.Join(repo.localPath, file.Name())
			repo.log.Debug().Str("filePath", filePath).Msg("Loading ConceptMap file")

			conceptMap, err := repo.loadConceptMapFile(filePath)
			if err != nil {
				repo.log.Error().
					Err(err).
					Str("file", file.Name()).
					Msg("Failed to load ConceptMap file")
				continue
			}

			if conceptMap.Id != nil {
				repo.cache.Store(*conceptMap.Id, conceptMap)
				repo.log.Debug().
					Str("id", *conceptMap.Id).
					Msg("Loaded ConceptMap into cache by ID")
			} else {
				repo.log.Warn().
					Str("file", file.Name()).
					Msg("ConceptMap has no ID")
			}

			if conceptMap.TargetUri != nil {
				repo.cache.Store(*conceptMap.TargetUri, conceptMap)
				repo.log.Debug().
					Str("targetUri", *conceptMap.TargetUri).
					Msg("Loaded ConceptMap into cache by TargetUri")
			} else {
				repo.log.Warn().
					Str("file", file.Name()).
					Msg("ConceptMap has no TargetUri")
			}
		}
	}

	repo.log.Info().Msg("Finished loading ConceptMaps from disk")
	return nil
}

// loadConceptMapFile loads a ConceptMap from a file.
func (repo *ConceptMapRepository) loadConceptMapFile(filePath string) (*fhir.ConceptMap, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ConceptMap file: %w", err)
	}

	var conceptMap fhir.ConceptMap
	if err := json.Unmarshal(data, &conceptMap); err != nil {
		return nil, fmt.Errorf("failed to parse ConceptMap: %w", err)
	}

	return &conceptMap, nil
}

// GetConceptMap retrieves a ConceptMap by ID or URL.
func (repo *ConceptMapRepository) GetConceptMap(key string) (*fhir.ConceptMap, error) {
	// Try cache first
	if cached, ok := repo.cache.Load(key); ok {
		return cached.(*fhir.ConceptMap), nil
	}

	// Load from disk
	filePath := filepath.Join(repo.localPath, repo.getFileName(key))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ConceptMap file: %w", err)
	}

	var conceptMap fhir.ConceptMap
	if err := json.Unmarshal(data, &conceptMap); err != nil {
		return nil, fmt.Errorf("failed to parse ConceptMap: %w", err)
	}
	repo.cache.Store(key, &conceptMap)
	return &conceptMap, nil
}

// GetConceptMapsByValuesetURL retrieves all ConceptMaps with a target URI matching the input URL.
func (repo *ConceptMapRepository) GetConceptMapsByValuesetURL(valueSetURL string) ([]*fhir.ConceptMap, error) {
	var matchingConceptMaps []*fhir.ConceptMap

	repo.cache.Range(func(key, value interface{}) bool {
		conceptMap := value.(*fhir.ConceptMap)
		if conceptMap.TargetUri != nil && *conceptMap.TargetUri == valueSetURL {
			matchingConceptMaps = append(matchingConceptMaps, conceptMap)
		}
		return true
	})

	if len(matchingConceptMaps) == 0 {
		repo.log.Warn().Str("valueSetURL", valueSetURL).Msg("No ConceptMaps found for ValueSet URL")
		return nil, fmt.Errorf("no ConceptMaps found for ValueSet URL: %s", valueSetURL)
	}

	return matchingConceptMaps, nil
}

// Helper function to get the file name for a ConceptMap by ID or URL.
func (repo *ConceptMapRepository) getFileName(key string) string {
	return fmt.Sprintf("%s.json", key)
}
