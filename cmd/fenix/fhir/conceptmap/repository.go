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

// TODO add source or target in function names to make it more clear what the function does
// ConceptMapRepository handles loading and storing ConceptMap resources.
type ConceptMapRepository struct {
	log         zerolog.Logger
	localPath   string
	cache       sync.Map
	conceptMaps map[string]fhir.ConceptMap
}

// NewConceptMapRepository creates a new ConceptMapRepository.
func NewConceptMapRepository(localPath string, log zerolog.Logger) *ConceptMapRepository {
	return &ConceptMapRepository{
		log:         log,
		localPath:   localPath,
		conceptMaps: make(map[string]fhir.ConceptMap),
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

			if conceptMap.TargetUri != nil {
				if *conceptMap.TargetUri != "" {
					repo.cache.Store(*conceptMap.TargetUri, conceptMap)
					repo.log.Debug().
						Str("targetUri", *conceptMap.TargetUri).
						Msg("Loaded ConceptMap into cache by TargetUri")
				}
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

// GetOrLoadConceptMap retrieves a ConceptMap by URL, loading it from disk if not already in cache.
func (repo *ConceptMapRepository) GetOrLoadConceptMap(url string) (*fhir.ConceptMap, error) {

	// Try cache first
	if cached, ok := repo.cache.Load(url); ok {
		return cached.(*fhir.ConceptMap), nil
	}

	// Load from disk if not in cache
	// TOOD: Ask why this is needed; because the cache should be loaded with all ConceptMaps if first in the main all conceptmaps
	// are loaded from disk and next all converted conceptmaps are also loaded from disk
	// It might however be usefull if you do not want to load all conceptmaps at once but only the ones you need (and load them only once)
	// Also the function GetConceptMapFileNameByURL uses readConceptMapFromFile  so it seems a bit circular...
	// Beacuse it needs to read the conceptmap to determine the filename. Soe maybe jsut let GetConceptMapFileNameByURL
	// return the conceptmap and not the filename
	fileName, err := repo.GetConceptMapFileNameByURL(url)
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(repo.localPath, fileName)

	conceptMap, err := repo.readConceptMapFromFile(filePath)
	if err != nil {
		repo.cache.Store(url, conceptMap)
		return conceptMap, nil
	} else {
		return nil, err
	}
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

// TODO check if this function is really needed, it seems not to be used anywhere
// Helper function to get the file name for a ConceptMap by ID or URL.
func (repo *ConceptMapRepository) getFileName(key string) string {
	return fmt.Sprintf("%s.json", key)
}

// GetConceptMapFileNameByURL returns the filename of a ConceptMap based on its URL
// This function is only called in the  GetOrLoadConceptMap function but I am not sure it is really needed there
// It might however be usefull if you do not want to have all conceptmaps loaded at once but only the ones you need
// TODO check if this function is really neededs
func (repo *ConceptMapRepository) GetConceptMapFileNameByURL(url string) (string, error) {
	var matchingFileName string
	err := filepath.Walk(repo.localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-JSON files
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		conceptMap, err := repo.readConceptMapFromFile(path)
		if err != nil {
			repo.log.Warn().
				Err(err).
				Str("file", info.Name()).
				Str("path", path).
				Msg("Failed to load ConceptMap file while searching")
			return nil // Continue walking even if one file fails
		}

		// TODO check if the targetUri check is a remnant of artdecor and should be removed?
		// As targeturi should be the url of the value set and not the url of the conceptmap...
		// TODO: check implications of stopping the walk when a match is found
		// Supposedly url names are unique so it should be fine, but do we make sure url names are unique?
		// Check both URL and TargetUri
		if (conceptMap.Url != nil && *conceptMap.Url == url) ||
			(conceptMap.TargetUri != nil && *conceptMap.TargetUri == url) {
			repo.log.Debug().
				Str("url", url).
				Str("filename", info.Name()).
				Msg("Found matching ConceptMap file")
			matchingFileName = info.Name()
			return filepath.SkipAll // Stop walking once we find a match
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error walking directory: %w", err)
	}

	if matchingFileName == "" {
		return "", fmt.Errorf("no ConceptMap file found for URL: %s", url)
	}

	return matchingFileName, nil
}

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
