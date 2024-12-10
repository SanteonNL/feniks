// cache.go
package valueset

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
)

type ValueSetService struct {
	cache      map[string]*CachedValueSet
	urlToPath  map[string]string
	mutex      sync.RWMutex
	localPath  string
	log        zerolog.Logger
	fhirClient *http.Client
}

func NewValueSetCache(localPath string, log zerolog.Logger) *ValueSetService {
	if err := os.MkdirAll(localPath, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create local storage directory")
	}

	cache := &ValueSetService{
		cache:      make(map[string]*CachedValueSet),
		urlToPath:  make(map[string]string),
		localPath:  localPath,
		log:        log,
		fhirClient: &http.Client{Timeout: 30 * time.Second},
	}

	if err := cache.loadAllFromDisk(); err != nil {
		log.Error().Err(err).Msg("Failed to load ValueSets from disk")
	}

	return cache
}

func (s *ValueSetService) saveToDisk(valueSetID string, valueSet *fhir.ValueSet) error {
	metadata := ValueSetMetadata{
		OriginalURL: valueSetID,
		ValueSet:    valueSet,
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ValueSet: %w", err)
	}

	fileName := fmt.Sprintf("%s.json", strings.ReplaceAll(valueSetID, "/", "_"))
	filePath := filepath.Join(s.localPath, fileName)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	s.mutex.Lock()
	s.urlToPath[valueSetID] = fileName
	s.mutex.Unlock()

	return nil
}
