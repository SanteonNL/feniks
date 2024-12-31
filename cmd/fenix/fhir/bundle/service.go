package bundle

import (
	"fmt"
	"net/http"

	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/rs/zerolog"
)

type BundleService struct {
	log zerolog.Logger
}

func NewBundleService(log zerolog.Logger) *BundleService {
	return &BundleService{
		log: log,
	}
}

// CreateSearchBundle creates a search result bundle
func (s *BundleService) CreateSearchBundle(req *http.Request, options SearchBundleOptions) *fhir.Bundle {
	bundle := &fhir.Bundle{
		Type:  fhir.BundleTypeSearchset,
		Total: 0,
		Entry: make([]fhir.BundleEntry, 0),
	}

	// Add self link
	if req != nil {
		bundle.Link = []fhir.BundleLink{{
			Relation: "self",
			Url:      s.getSelfLink(req),
		}}
	}

	// Handle error case first
	if options.Outcome != nil {
		bundle.Entry = append(bundle.Entry, fhir.BundleEntry{
			Resource: options.Outcome,
			Search: &fhir.BundleEntrySearch{
				Mode: fhir.SearchEntryModeOutcome,
			},
		})
		return bundle
	}

	// Add resources if present
	if options.Resources != nil {
		for _, resource := range options.Resources {
			entry := s.createResourceEntry(resource, req)
			bundle.Entry = append(bundle.Entry, entry)
		}
		bundle.Total = len(options.Resources)
	}

	return bundle
}

// CreateTransactionBundle creates a transaction/batch bundle
func (s *BundleService) CreateTransactionBundle(req *http.Request, options TransactionBundleOptions) *fhir.Bundle {
	bundle := &fhir.Bundle{
		Type:  options.Type, // BundleTypeTransaction or BundleTypeBatch
		Entry: make([]fhir.BundleEntry, 0),
	}

	// Add transaction/batch specific entries...

	return bundle
}

// Options structs to make bundle creation more flexible
type SearchBundleOptions struct {
	Resources []interface{}
	Outcome   *fhir.OperationOutcome
}

type TransactionBundleOptions struct {
	Type     fhir.BundleType
	Requests []fhir.BundleEntry
}

// Helper methods
func (s *BundleService) createResourceEntry(resource interface{}, req *http.Request) fhir.BundleEntry {
	entry := fhir.BundleEntry{
		Search: &fhir.BundleEntrySearch{
			Mode: &fhir.SearchEntryModeMatch,
		},
	}

	// Add fullUrl if possible
	if id := s.getResourceID(resource); id != "" && req != nil {
		entry.FullUrl = fmt.Sprintf("%s/%s", s.getBaseUrl(req), id)
	}

	return entry
}

func (s *BundleService) getSelfLink(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, r.Host, r.URL.RequestURI())
}

func (s *BundleService) getBaseUrl(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/fhir", scheme, r.Host)
}

func (s *BundleService) getResourceID(resource interface{}) string {
	switch r := resource.(type) {
	case map[string]interface{}:
		if id, ok := r["id"].(string); ok {
			return id
		}
	case fhir.Resource:
		if id := r.GetID(); id != nil {
			return *id
		}
	}
	return ""
}

// CreateOutcomeBundle creates a search bundle with just an OperationOutcome
func (s *BundleService) CreateOutcomeBundle(outcome *fhir.OperationOutcome) *fhir.Bundle {
	bundle := &fhir.Bundle{
		Type:  fhir.BundleTypeSearchset,
		Total: 0,
		Entry: make([]fhir.BundleEntry, 0),
	}

	if outcome != nil {
		bundle.Entry = append(bundle.Entry, fhir.BundleEntry{

			Search: &fhir.BundleEntrySearch{
				Mode: 2,
			},
		})
	}

	return bundle
}
