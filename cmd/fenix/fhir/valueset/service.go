// service.go
package valueset

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
)

func (s *ValueSetService) GetValueSet(ctx context.Context, url string) (*fhir.ValueSet, error) {
	valueSetID, source := s.parseValueSetURL(url)

	s.log.Debug().
		Str("originalURL", url).
		Str("valueSetID", valueSetID).
		Str("source", source.String()).
		Msg("Resolving ValueSet source")

	s.mutex.RLock()
	cached, exists := s.cache[valueSetID]
	s.mutex.RUnlock()

	if exists {
		return cached.ValueSet, nil
	}

	var valueSet *fhir.ValueSet
	var err error

	switch source {
	case LocalSource:
		valueSet, err = s.fetchFromLocal(valueSetID)
	case RemoteSource:
		valueSet, err = s.fetchFromRemote(ctx, valueSetID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch ValueSet: %w", err)
	}

	s.mutex.Lock()
	s.cache[valueSetID] = &CachedValueSet{
		ValueSet:    valueSet,
		LastChecked: time.Now(),
	}
	s.mutex.Unlock()

	return valueSet, nil
}

func (s *ValueSetService) ValidateCode(ctx context.Context, valueSetURL string, coding *fhir.Coding) (*ValidationResult, error) {
	processedURLs := sync.Map{}
	return s.validateCodeRecursive(ctx, valueSetURL, coding, &processedURLs)
}

func (s *ValueSetService) validateCodeRecursive(ctx context.Context, valueSetURL string, coding *fhir.Coding, processedURLs *sync.Map) (*ValidationResult, error) {
	if _, exists := processedURLs.Load(valueSetURL); exists {
		return nil, fmt.Errorf("circular reference detected in ValueSet: %s", valueSetURL)
	}
	processedURLs.Store(valueSetURL, true)

	// Get the ValueSet
	valueSet, err := s.GetValueSet(ctx, valueSetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ValueSet %s: %w", valueSetURL, err)
	}

	// First check the direct concepts in this ValueSet
	result := s.validateDirectConcepts(valueSet, coding)
	if result.Valid {
		return result, nil
	}

	// If no direct match found and compose section exists, check included ValueSets
	if valueSet.Compose != nil {
		// Create a channel for results from included ValueSets
		type includeResult struct {
			result *ValidationResult
			err    error
		}
		results := make(chan includeResult)

		// Process each include in parallel
		var wg sync.WaitGroup
		for _, include := range valueSet.Compose.Include {
			if include.ValueSet == nil || len(include.ValueSet) == 0 {
				continue
			}

			for _, includeValueSetURL := range include.ValueSet {
				wg.Add(1)
				go func(url string) {
					defer wg.Done()
					res, err := s.validateCodeRecursive(ctx, url, coding, processedURLs)
					results <- includeResult{result: res, err: err}
				}(includeValueSetURL)
			}
		}

		// Close the results channel when all goroutines are done
		go func() {
			wg.Wait()
			close(results)
		}()

		// Collect results
		for res := range results {
			if res.err != nil {
				s.log.Warn().Err(res.err).Msg("Error validating code in included ValueSet")
				continue
			}
			if res.result.Valid {
				return res.result, nil
			}
		}
	}

	return &ValidationResult{
		Valid:        false,
		ErrorMessage: fmt.Sprintf("Code not found in ValueSet %s", valueSetURL),
	}, nil
}

func (s *ValueSetService) validateDirectConcepts(valueSet *fhir.ValueSet, coding *fhir.Coding) *ValidationResult {
	var codingSystem, codingCode string
	if coding.System != nil {
		codingSystem = *coding.System
	}
	if coding.Code != nil {
		codingCode = *coding.Code
	}

	for _, include := range valueSet.Compose.Include {
		if include.System != nil && *include.System != codingSystem {
			continue
		}

		for _, concept := range include.Concept {
			if concept.Code == codingCode {
				return &ValidationResult{
					Valid:     true,
					MatchedIn: *valueSet.Url,
				}
			}
		}
	}

	return &ValidationResult{
		Valid: false,
	}
}

func (s *ValueSetService) parseValueSetURL(url string) (string, ValueSetSource) {
	url = strings.TrimPrefix(url, "ValueSet/")
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url, RemoteSource
	}
	return url, LocalSource
}
