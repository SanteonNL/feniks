// types.go
package valueset

import (
	"time"

	"github.com/SanteonNL/fenix/models/fhir"
)

type CachedValueSet struct {
	ValueSet    *fhir.ValueSet
	LastChecked time.Time
}

type ValueSetMetadata struct {
	OriginalURL string         `json:"originalUrl"`
	ValueSet    *fhir.ValueSet `json:"valueSet"`
}

type ValidationResult struct {
	Valid        bool
	MatchedIn    string
	ErrorMessage string
}

type ValueSetSource int

const (
	LocalSource ValueSetSource = iota
	RemoteSource
)

func (v ValueSetSource) String() string {
	switch v {
	case LocalSource:
		return "LocalSource"
	case RemoteSource:
		return "RemoteSource"
	default:
		return "UnknownSource"
	}
}
