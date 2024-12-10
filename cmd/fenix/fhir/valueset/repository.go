// repository.go
package valueset

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
)

func (s *ValueSetService) loadAllFromDisk() error {
	files, err := os.ReadDir(s.localPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			filePath := filepath.Join(s.localPath, file.Name())
			valueSet, err := s.loadValueSetFromDisk(filePath)
			if err != nil {
				s.log.Error().
					Err(err).
					Str("file", file.Name()).
					Msg("Failed to load ValueSet from disk")
				continue
			}

			if valueSet == nil || valueSet.Url == nil {
				s.log.Warn().
					Str("file", file.Name()).
					Msg("ValueSet missing or ValueSet URL missing")
				continue
			}

			s.mutex.Lock()
			s.cache[*valueSet.Url] = &CachedValueSet{
				ValueSet:    valueSet,
				LastChecked: time.Now(),
			}
			s.urlToPath[*valueSet.Url] = file.Name()
			s.mutex.Unlock()
		}
	}

	return nil
}

func (s *ValueSetService) loadValueSetFromDisk(filePath string) (*fhir.ValueSet, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var metadata ValueSetMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		// Try loading as plain ValueSet for backwards compatibility
		var valueSet fhir.ValueSet
		if err := json.Unmarshal(data, &valueSet); err != nil {
			return nil, fmt.Errorf("failed to parse ValueSet: %w", err)
		}
		return &valueSet, nil
	}

	return metadata.ValueSet, nil
}

func (s *ValueSetService) fetchFromLocal(valueSetID string) (*fhir.ValueSet, error) {
	s.mutex.RLock()
	fileName, exists := s.urlToPath[valueSetID]
	s.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no local file mapping found for ValueSet: %s", valueSetID)
	}

	filePath := filepath.Join(s.localPath, fileName)
	return s.loadValueSetFromDisk(filePath)
}

func (s *ValueSetService) fetchFromRemote(ctx context.Context, url string) (*fhir.ValueSet, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Accept", "application/json")

	resp, err := s.fhirClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var valueSet fhir.ValueSet
	if err := json.Unmarshal(bodyBytes, &valueSet); err != nil {
		return nil, fmt.Errorf("failed to decode ValueSet: %w", err)
	}

	return &valueSet, nil
}
