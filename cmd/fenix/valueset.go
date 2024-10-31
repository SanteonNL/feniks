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

type ValueSetCache struct {
	cache      map[string]*CachedValueSet
	mutex      sync.RWMutex
	fhirClient *http.Client
	localPath  string
	log        zerolog.Logger
}

type CachedValueSet struct {
	ValueSet    *fhir.ValueSet
	LastChecked time.Time // Internal tracking only
}

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
		fhirClient: &http.Client{Timeout: 30 * time.Second},
		localPath:  localPath,
		log:        log,
	}

	// Load existing ValueSets
	if err := cache.loadAllFromDisk(); err != nil {
		log.Error().Err(err).Msg("Failed to load ValueSets from disk")
	}

	return cache
}

func (vc *ValueSetCache) parseValueSetURL(url string) (string, ValueSetSource) {
	// Remove any "ValueSet/" prefix
	url = strings.TrimPrefix(url, "ValueSet/")

	// If it starts with http(s), it's a remote source
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url, RemoteSource
	}

	// Otherwise, it's a local source
	return url, LocalSource
}

func (vc *ValueSetCache) GetValueSet(url string) (*fhir.ValueSet, error) {
	valueSetID, source := vc.parseValueSetURL(url)

	vc.log.Debug().
		Str("originalURL", url).
		Str("valueSetID", valueSetID).
		Str("source", source.String()).
		Msg("Resolving ValueSet source")

	vc.mutex.RLock()
	cached, exists := vc.cache[valueSetID]
	vc.mutex.RUnlock()

	if exists {
		var lastUpdated time.Time
		if cached.ValueSet.Meta != nil && cached.ValueSet.Meta.LastUpdated != nil {
			lastUpdated = cached.ValueSet.Meta.LastUpdated.Time
		}

		if time.Since(lastUpdated) > 24*time.Hour &&
			time.Since(cached.LastChecked) > 1*time.Hour {
			go vc.updateValueSet(valueSetID, source)
		}
		return cached.ValueSet, nil
	}

	return vc.updateValueSet(valueSetID, source)
}

func (vc *ValueSetCache) loadAllFromDisk() error {
	files, err := os.ReadDir(vc.localPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			valueSetID := strings.TrimSuffix(file.Name(), ".json")

			valueSet, err := vc.loadFromDisk(valueSetID)
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
			vc.cache[valueSetID] = &CachedValueSet{
				ValueSet:    valueSet,
				LastChecked: time.Now(),
			}
			vc.mutex.Unlock()
		}
	}

	vc.log.Info().
		Int("loadedCount", len(vc.cache)).
		Msg("Loaded ValueSets from disk")
	return nil
}

func (vc *ValueSetCache) updateValueSet(valueSetID string, source ValueSetSource) (*fhir.ValueSet, error) {
	vc.mutex.Lock()
	defer vc.mutex.Unlock()

	var valueSet *fhir.ValueSet
	var err error

	switch source {
	case LocalSource:
		valueSet, err = vc.fetchFromLocal(valueSetID)
	case RemoteSource:
		valueSet, err = vc.fetchFromRemote(valueSetID)
	}

	if err != nil {
		// Try to get from cache if fetch fails
		if cached, exists := vc.cache[valueSetID]; exists {
			cached.LastChecked = time.Now()
			vc.log.Warn().
				Err(err).
				Str("valueSetID", valueSetID).
				Msg("Fetch failed, using cached version")
			return cached.ValueSet, nil
		}

		// Try local storage as last resort
		valueSet, err = vc.loadFromDisk(valueSetID)
		if err != nil {
			return nil, fmt.Errorf("failed to load ValueSet from all sources: %w", err)
		}
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
	vc.cache[valueSetID] = &CachedValueSet{
		ValueSet:    valueSet,
		LastChecked: now,
	}

	// Save to disk
	if err := vc.saveToDisk(valueSetID, valueSet); err != nil {
		vc.log.Error().
			Err(err).
			Str("valueSetID", valueSetID).
			Msg("Failed to save ValueSet to disk")
	}

	return valueSet, nil
}

func (vc *ValueSetCache) fetchFromLocal(valueSetID string) (*fhir.ValueSet, error) {
	return vc.loadFromDisk(valueSetID)
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

// Update the storage functions to use safe filenames
func (vc *ValueSetCache) loadFromDisk(valueSetID string) (*fhir.ValueSet, error) {
	safeID := vc.safeFileName(valueSetID)
	vsPath := filepath.Join(vc.localPath, fmt.Sprintf("%s.json", safeID))

	vc.log.Debug().
		Str("originalID", valueSetID).
		Str("safeID", safeID).
		Str("path", vsPath).
		Msg("Loading ValueSet from disk")

	data, err := os.ReadFile(vsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var valueSet fhir.ValueSet
	if err := json.Unmarshal(data, &valueSet); err != nil {
		return nil, fmt.Errorf("failed to parse ValueSet: %w", err)
	}

	return &valueSet, nil
}

func (vc *ValueSetCache) saveToDisk(valueSetID string, vs *fhir.ValueSet) error {
	safeID := vc.safeFileName(valueSetID)
	vsPath := filepath.Join(vc.localPath, fmt.Sprintf("%s.json", safeID))

	vc.log.Debug().
		Str("originalID", valueSetID).
		Str("safeID", safeID).
		Str("path", vsPath).
		Msg("Saving ValueSet to disk")

	data, err := json.MarshalIndent(vs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ValueSet: %w", err)
	}

	if err := os.WriteFile(vsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// safeFileName converts a URL to a safe filename while maintaining uniqueness
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
	return fmt.Sprintf("%s-%s", name, hash)
}

// Add this method to your ValueSetCache struct
func (vc *ValueSetCache) SetTimeout(duration time.Duration) {
	vc.log.Info().
		Str("timeout", duration.String()).
		Msg("Setting client timeout")
	vc.fhirClient.Timeout = duration
}
