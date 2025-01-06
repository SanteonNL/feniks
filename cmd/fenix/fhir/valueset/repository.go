// repository.go
package valueset

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (s *ValueSetService) loadAllFromDisk() error {
	// First load URL mappings
	if err := s.loadURLMappings(); err != nil {
		s.log.Error().Err(err).Msg("Failed to load URL mappings")
	}

	files, err := os.ReadDir(s.localPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") && file.Name() != "url-mappings.json" {
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

			// Only cache if not expired
			if !s.isLocalStorageExpired(*valueSet.Url) {
				s.updateCache(*valueSet.Url, valueSet)
			}
		}
	}

	return nil
}
