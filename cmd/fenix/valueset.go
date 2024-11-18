package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
)

type CachedValueSet struct {
	ValueSet    *fhir.ValueSet
	LastChecked time.Time // Internal tracking only
}

type ValueSetCache struct {
	cache      map[string]*CachedValueSet
	urlToPath  map[string]string
	mutex      sync.RWMutex
	localPath  string
	log        zerolog.Logger
	fhirClient *http.Client
}

// These constants and types were in the original but not shown in the refactor
type ValueSetSource int

const (
	LocalSource ValueSetSource = iota
	RemoteSource
)

func (s ValueSetSource) String() string {
	switch s {
	case LocalSource:
		return "local"
	case RemoteSource:
		return "remote"
	default:
		return "unknown"
	}
}

// NewValueSetCache creates a new cache instance
func NewValueSetCache(localPath string, log zerolog.Logger) *ValueSetCache {
	// Create local storage directory if it doesn't exist
	if err := os.MkdirAll(localPath, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create local storage directory")
	}

	cache := &ValueSetCache{
		cache:      make(map[string]*CachedValueSet),
		urlToPath:  make(map[string]string),
		localPath:  localPath,
		log:        log,
		fhirClient: &http.Client{Timeout: 30 * time.Second},
	}

	// Load existing ValueSets
	if err := cache.loadAllFromDisk(); err != nil {
		log.Error().Err(err).Msg("Failed to load ValueSets from disk")
	}

	return cache
}

func (vc *ValueSetCache) GetValueSet(url string) (*fhir.ValueSet, error) {
	valueSetID, source := vc.parseValueSetURL(url)

	vc.log.Debug().
		Str("originalURL", url).
		Str("valueSetID", valueSetID).
		Str("source", source.String()).
		Msg("Resolving ValueSet source")

	// Try to get from cache first
	vc.mutex.RLock()
	cached, exists := vc.cache[valueSetID]
	vc.mutex.RUnlock()

	if exists {
		var lastUpdated time.Time
		if cached.ValueSet.Meta != nil && cached.ValueSet.Meta.LastUpdated != nil {
			lastUpdated = cached.ValueSet.Meta.LastUpdated.Time
		}

		// Check if we need to update in background
		if time.Since(lastUpdated) > 24*time.Hour &&
			time.Since(cached.LastChecked) > 1*time.Hour {
			go func() {
				_, err := vc.updateValueSet(valueSetID, source)
				if err != nil {
					vc.log.Error().Err(err).Msg("Background update failed")
				}
			}()
		}
		return cached.ValueSet, nil
	}

	return vc.updateValueSet(valueSetID, source)
}

func (vc *ValueSetCache) updateValueSet(valueSetID string, source ValueSetSource) (*fhir.ValueSet, error) {
	var valueSet *fhir.ValueSet
	var err error

	// Fetch from appropriate source
	switch source {
	case LocalSource:
		valueSet, err = vc.fetchFromLocal(valueSetID)
	case RemoteSource:
		valueSet, err = vc.fetchFromRemote(valueSetID)
	}

	if err != nil {
		// Try to get from cache if fetch fails
		vc.mutex.RLock()
		cached, exists := vc.cache[valueSetID]
		vc.mutex.RUnlock()

		if exists {
			vc.mutex.Lock()
			cached.LastChecked = time.Now()
			vc.mutex.Unlock()

			vc.log.Warn().
				Err(err).
				Str("valueSetID", valueSetID).
				Msg("Fetch failed, using cached version")
			return cached.ValueSet, nil
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch ValueSet: %w", err)
	}

	// Ensure metadata is set
	now := time.Now()
	if valueSet.Meta == nil {
		valueSet.Meta = &fhir.Meta{}
	}
	if valueSet.Meta.LastUpdated == nil {
		valueSet.Meta.LastUpdated = &fhir.DateTime{Time: now}
	}

	// Update cache
	vc.mutex.Lock()
	vc.cache[valueSetID] = &CachedValueSet{
		ValueSet:    valueSet,
		LastChecked: now,
	}
	vc.mutex.Unlock()

	// Save to disk asynchronously
	go func() {
		if err := vc.saveToDisk(valueSetID, valueSet); err != nil {
			vc.log.Error().
				Err(err).
				Str("valueSetID", valueSetID).
				Msg("Failed to save ValueSet to disk")
		}
	}()

	return valueSet, nil
}

func (vc *ValueSetCache) fetchFromRemote(url string) (*fhir.ValueSet, error) {
	vc.log.Debug().
		Str("url", url).
		Msg("Fetching ValueSet from remote server")

	resp, err := vc.fhirClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var valueSet fhir.ValueSet
	if err := json.NewDecoder(resp.Body).Decode(&valueSet); err != nil {
		return nil, fmt.Errorf("failed to decode ValueSet: %w", err)
	}

	return &valueSet, nil
}

// The local fetching function
func (vc *ValueSetCache) fetchFromLocal(valueSetID string) (*fhir.ValueSet, error) {
	vc.mutex.RLock()
	fileName, exists := vc.urlToPath[valueSetID]
	vc.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no local file mapping found for ValueSet: %s", valueSetID)
	}

	valueSet, originalURL, err := vc.loadValueSetWithMetadata(
		filepath.Join(vc.localPath, fileName))
	if err != nil {
		return nil, err
	}

	if originalURL != valueSetID {
		vc.log.Warn().
			Str("expectedID", valueSetID).
			Str("foundID", originalURL).
			Msg("ValueSet ID mismatch in local file")
	}

	return valueSet, nil
}

// The original safeFileName function (still needed for new files)
func (vc *ValueSetCache) safeFileName(url string) string {
	// Create a hash of the original URL to ensure uniqueness
	hasher := sha256.New()
	hasher.Write([]byte(url))
	hash := hex.EncodeToString(hasher.Sum(nil))[:12] // First 12 chars of hash should be enough

	// Create a readable prefix from the URL
	// Remove common prefixes
	name := strings.TrimPrefix(url, "http://")
	name = strings.TrimPrefix(name, "https://")
	name = strings.TrimPrefix(name, "ValueSet/")

	// Replace problematic characters
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
		".", "_",
	)
	name = replacer.Replace(name)

	// Limit the length of the readable part
	if len(name) > 50 {
		name = name[:50]
	}

	// Combine readable name with hash
	return fmt.Sprintf("%s-%s.json", name, hash)
}

func (vc *ValueSetCache) SetTimeout(duration time.Duration) {
	vc.log.Info().
		Str("timeout", duration.String()).
		Msg("Setting client timeout")
	vc.fhirClient.Timeout = duration
}

func (vc *ValueSetCache) parseValueSetURL(url string) (string, ValueSetSource) {
	url = strings.TrimPrefix(url, "ValueSet/")
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url, RemoteSource
	}
	return url, LocalSource
}
func (vc *ValueSetCache) loadAllFromDisk() error {
	files, err := os.ReadDir(vc.localPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Load the URL mapping file if it exists
	if err := vc.loadURLMapping(); err != nil {
		vc.log.Debug().Err(err).Msg("Failed to load existing URL mapping, will rebuild from metadata")
		// Continue with empty mapping, will rebuild from metadata
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") && file.Name() != "url_mapping.json" {
			filePath := filepath.Join(vc.localPath, file.Name())

			valueSet, originalURL, err := vc.loadValueSetWithMetadata(filePath)
			if err != nil {
				vc.log.Error().
					Err(err).
					Str("file", file.Name()).
					Msg("Failed to load ValueSet from disk")
				continue
			}

			// Initialize metadata if needed
			if valueSet.Meta == nil {
				valueSet.Meta = &fhir.Meta{}
			}
			if valueSet.Meta.LastUpdated == nil {
				valueSet.Meta.LastUpdated = &fhir.DateTime{Time: time.Now()}
			}

			vc.mutex.Lock()
			vc.cache[originalURL] = &CachedValueSet{
				ValueSet:    valueSet,
				LastChecked: time.Now(),
			}
			// Store the filename without full path
			vc.urlToPath[originalURL] = file.Name()
			vc.mutex.Unlock()
		}
	}

	// Save the URL mapping in case it was rebuilt
	if err := vc.saveURLMapping(); err != nil {
		vc.log.Warn().Err(err).Msg("Failed to save rebuilt URL mapping")
	}

	vc.log.Info().
		Int("loadedCount", len(vc.cache)).
		Int("mappingCount", len(vc.urlToPath)).
		Msg("Loaded ValueSets from disk")
	return nil
}

type ValueSetMetadata struct {
	OriginalURL string         `json:"originalUrl"`
	ValueSet    *fhir.ValueSet `json:"valueSet"`
}

func (vc *ValueSetCache) loadValueSetWithMetadata(filePath string) (*fhir.ValueSet, string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %w", err)
	}

	// First try to load as metadata format
	var metadata ValueSetMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		// If that fails, try loading as plain ValueSet (backwards compatibility)
		var valueSet fhir.ValueSet
		if err := json.Unmarshal(data, &valueSet); err != nil {
			return nil, "", fmt.Errorf("failed to parse ValueSet or metadata: %w", err)
		}

		// For backwards compatibility, try to extract ID from filename
		baseFileName := filepath.Base(filePath)
		originalURL := strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName))

		return &valueSet, originalURL, nil
	}

	if metadata.ValueSet == nil {
		return nil, "", fmt.Errorf("metadata contains nil ValueSet")
	}

	return metadata.ValueSet, metadata.OriginalURL, nil
}

func (vc *ValueSetCache) saveToDisk(valueSetID string, vs *fhir.ValueSet) error {
	vc.mutex.Lock()
	fileName, exists := vc.urlToPath[valueSetID]
	if !exists {
		// Create new filename only if it doesn't exist
		fileName = vc.safeFileName(valueSetID)
		vc.urlToPath[valueSetID] = fileName
	}
	vc.mutex.Unlock()

	vsPath := filepath.Join(vc.localPath, fileName)

	metadata := ValueSetMetadata{
		OriginalURL: valueSetID,
		ValueSet:    vs,
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ValueSet metadata: %w", err)
	}

	if err := os.WriteFile(vsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Save the updated URL mapping
	return vc.saveURLMapping()
}

func (vc *ValueSetCache) loadURLMapping() error {
	mappingPath := filepath.Join(vc.localPath, "url_mapping.json")
	data, err := os.ReadFile(mappingPath)
	if err != nil {
		if os.IsNotExist(err) {
			// First time initialization - not an error
			vc.log.Debug().Msg("URL mapping file doesn't exist yet, will create on first save")
			return nil
		}
		return fmt.Errorf("failed to read URL mapping file: %w", err)
	}

	vc.mutex.Lock()
	defer vc.mutex.Unlock()

	if err := json.Unmarshal(data, &vc.urlToPath); err != nil {
		return fmt.Errorf("failed to parse URL mapping file: %w", err)
	}

	return nil
}

func (vc *ValueSetCache) saveURLMapping() error {
	mappingPath := filepath.Join(vc.localPath, "url_mapping.json")

	vc.mutex.RLock()
	data, err := json.MarshalIndent(vc.urlToPath, "", "  ")
	vc.mutex.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to marshal URL mapping: %w", err)
	}

	return os.WriteFile(mappingPath, data, 0644)
}
