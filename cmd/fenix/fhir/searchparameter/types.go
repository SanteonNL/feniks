package searchparameter

import (
	"sync"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
)

type SearchParameterRepository struct {
	searchParametersMap map[string]*fhir.SearchParameter // URL -> SearchParameter
	mu                  sync.RWMutex
	log                 zerolog.Logger
}

type SearchParamInfo struct {
	Type string
	Code string
	Base []string
}

// SearchParameterService manages search parameter operations and indexing
type SearchParameterService struct {
	repo        *SearchParameterRepository
	log         zerolog.Logger
	pathCodeMap map[string]map[string]string
	mu          sync.RWMutex
}
