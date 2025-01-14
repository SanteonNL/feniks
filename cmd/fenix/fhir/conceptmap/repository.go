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

// TODO: think about when the cache should be cleared and how to do this
// TODO: think about cache versus loading conceptmaps and how to do it comparable to the valueset repository
// ConceptMapRepository handles loading and storing ConceptMap resources.
type ConceptMapRepository struct {
	log       zerolog.Logger
	localPath string
	cache     sync.Map
	//conceptMaps map[string]fhir.ConceptMap
}

// NewConceptMapRepository creates a new ConceptMapRepository.
func NewConceptMapRepository(localPath string, log zerolog.Logger) *ConceptMapRepository {
	return &ConceptMapRepository{
		log:       log,
		localPath: localPath,
		//conceptMaps: make(map[string]fhir.ConceptMap),
	}
}

// LoadConceptMapsIntoRepository loads all ConceptMaps into the repository.
func (repo *ConceptMapRepository) LoadConceptMapsIntoRepository() error {
	if _, err := os.Stat(repo.localPath); os.IsNotExist(err) {
		repo.log.Warn().Str("path", repo.localPath).Msg("ConceptMap directory does not exist")
		return nil
	}

	files, err := os.ReadDir(repo.localPath)
	if err != nil {
		repo.log.Error().Err(err).Msg("Failed to read directory")
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			filePath := filepath.Join(repo.localPath, file.Name())
			repo.log.Debug().Str("filePath", filePath).Msg("Loading ConceptMap file")

			conceptMap, err := repo.readConceptMapFromFile(filePath)
			if err != nil {
				repo.log.Error().
					Err(err).
					Str("file", file.Name()).
					Msg("Failed to load ConceptMap file")
				continue
			}

			// TODO: add check for empty string or make sure that the url is always present
			// Also check this at other places where the url is used
			if conceptMap.Url != nil {
				repo.cache.Store(*conceptMap.Url, conceptMap)
				repo.log.Debug().
					Str("id", *conceptMap.Url).
					Msg("Loaded ConceptMap into cache by Url")
			} else {
				repo.log.Warn().
					Str("file", file.Name()).
					Msg("ConceptMap has no Url")
			}
		}
	}

	repo.log.Info().Msg("Finished loading ConceptMaps from disk")
	return nil
}

// readConceptMapFile reads a ConceptMap from a file.
func (repo *ConceptMapRepository) readConceptMapFromFile(filePath string) (*fhir.ConceptMap, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// TODO check of tis should be fatal or Error (in the GetOrLoadConceptMap function it is an error)
		repo.log.Fatal().Err(err).Str("path", filePath).Msg("Failed to read ConceptMap file")
		return nil, fmt.Errorf("failed to read ConceptMap file: %w", err)
	}

	var conceptMap fhir.ConceptMap
	if err := json.Unmarshal(data, &conceptMap); err != nil {
		return nil, fmt.Errorf("failed to parse ConceptMap: %w", err)
	}

	return &conceptMap, nil
}

// GetConceptMap retrieves a ConceptMap from cache by URL
// TODO: maybe add back also loading from disk if not in cache if needed
func (repo *ConceptMapRepository) GetConceptMap(url string) (*fhir.ConceptMap, error) {

	if cached, ok := repo.cache.Load(url); ok {
		return cached.(*fhir.ConceptMap), nil
	}

	return nil, fmt.Errorf("ConceptMap not found in cache for URL: %s", url)
}

// GetConceptMapURLsByValuesetURL retrieves all ConceptMaps with a target URI matching the input URL.
func (repo *ConceptMapRepository) GetConceptMapURLsByValuesetURL(valueSetURL string) ([]string, error) {
	var matchingConceptMapURLs []string

	repo.cache.Range(func(key, value interface{}) bool {
		repo.log.Debug().Str("key", key.(string)).Msg("Checking ConceptMap for ValueSet URL")
		conceptMap := value.(*fhir.ConceptMap)
		if conceptMap.TargetUri != nil && *conceptMap.TargetUri == valueSetURL {
			repo.log.Debug().Str("Adding ConceptMap URL", *conceptMap.Url).Msg("Found matching ConceptMap")
			matchingConceptMapURLs = append(matchingConceptMapURLs, *conceptMap.Url)
		}
		return true
	})

	if len(matchingConceptMapURLs) == 0 {
		repo.log.Warn().Str("valueSetURL", valueSetURL).Msg("No ConceptMapURLs found for ValueSet URL")
		return nil, fmt.Errorf("no ConceptMapURLs found for ValueSet URL: %s", valueSetURL)
	}

	return matchingConceptMapURLs, nil
}

// TODO: remove function and call in main when done?
// GetAllConceptMapsFromCache returns all ConceptMaps in the repository, this is just used to check what is in cache
// and not really needed functionality
func (r *ConceptMapRepository) GetAllConceptMapsFromCache() []string {
	var keys []string
	r.cache.Range(func(key, value interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})

	return keys

}
