package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/SanteonNL/fenix/cmd/fenix/fhir/searchparameter"
	"github.com/SanteonNL/fenix/cmd/fenix/processor"
	"github.com/SanteonNL/fenix/cmd/fenix/types"
	"github.com/SanteonNL/fenix/models/fhir"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type FHIRRouter struct {
	searchParamService *searchparameter.SearchParameterService
	processorService   *processor.ProcessorService
}

func NewFHIRRouter(searchParamService *searchparameter.SearchParameterService, processorService *processor.ProcessorService) *FHIRRouter {
	return &FHIRRouter{
		searchParamService: searchParamService,
		processorService:   processorService,
	}
}

func (fr *FHIRRouter) SetupRoutes() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// FHIR routes
	r.Route("/r4", func(r chi.Router) {
		// Metadata endpoint (CapabilityStatement)
		// r.Get("/metadata", fr.handleMetadata)

		// Dynamic resource type routes
		r.Route("/{resourceType}", func(r chi.Router) {
			// Type level interactions
			r.Get("/", fr.handleSearch) // Search

			// TODO: Add other id level interactions
			// Instance level interactions
			// r.Route("/{id}", func(r chi.Router) {
			// 	r.Get("/", fr.handleRead) // Read
			// })
		})
	})

	return r
}

func (fr *FHIRRouter) handleSearch(w http.ResponseWriter, r *http.Request) {
	resourceType := chi.URLParam(r, "resourceType")

	// Validate resource type exists in factory
	if !isValidResourceType(resourceType) {
		respondWithOperationOutcome(w, http.StatusNotFound, "error", "not-found",
			fmt.Sprintf("Resource type %s is not supported", resourceType))
		return
	}

	// Get and validate search parameters
	queryParams := r.URL.Query()
	_, invalidFilters := fr.validateSearchParameters(resourceType, queryParams)

	// If there are invalid parameters, return error response
	if len(invalidFilters) > 0 {
		outcome := createOperationOutcome(invalidFilters)
		respondWithJSON(w, http.StatusBadRequest, outcome)
		return
	}

	// // Process search with valid filters
	// results, err := fr.processorService.ProcessResources(r.Context(), resourceType, validFilters)
	// if err != nil {
	// 	respondWithOperationOutcome(w, http.StatusInternalServerError, "error", "processing",
	// 		fmt.Sprintf("Error processing search: %v", err))
	// 	return
	// }

	// // Create and return bundle
	// bundle := createSearchBundle(results, nil)
	//respondWithJSON(w, http.StatusOK, outcome)
}

func (fr *FHIRRouter) validateSearchParameters(resourceType string, params map[string][]string) ([]*types.Filter, []*types.Filter) {
	var validFilters, invalidFilters []*types.Filter

	for paramName, values := range params {
		// Split parameter name and modifier (e.g., "code:in" -> "code", "in")
		baseParam, modifier := splitParameter(paramName)

		// Validate using SearchParameterService
		filter, err := fr.searchParamService.ValidateSearchParameter(resourceType, baseParam, modifier)

		if err != nil || !filter.IsValid {
			filter.Value = values[0] // Add value for error reporting
			invalidFilters = append(invalidFilters, filter)
			continue
		}

		// Add value to valid filter
		filter.Value = values[0] // Handle multiple values if needed
		validFilters = append(validFilters, filter)
	}

	return validFilters, invalidFilters
}

// Helper functions

func splitParameter(param string) (string, string) {
	parts := strings.Split(param, ":")
	if len(parts) > 1 {
		return parts[0], parts[1]
	}
	return param, ""
}

func isValidResourceType(resourceType string) bool {
	_, exists := processor.ResourceFactoryMap[resourceType]
	return exists
}

func respondWithJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/fhir+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondWithOperationOutcome(w http.ResponseWriter, status int, severity string, code string, diagnostics string) {
	// Convert string to IssueSeverity type
	var issueSeverity fhir.IssueSeverity
	if err := issueSeverity.UnmarshalJSON([]byte(`"` + severity + `"`)); err != nil {
		// Default to error if unmarshal fails
		issueSeverity = fhir.IssueSeverityError
	}

	// Convert string to IssueType
	var issueType fhir.IssueType
	if err := issueType.UnmarshalJSON([]byte(`"` + code + `"`)); err != nil {
		// Default to processing if unmarshal fails
		issueType = fhir.IssueTypeProcessing
	}

	outcome := &fhir.OperationOutcome{
		Issue: []fhir.OperationOutcomeIssue{
			{
				Severity:    issueSeverity,
				Code:        issueType,
				Diagnostics: &diagnostics,
			},
		},
	}

	respondWithJSON(w, status, outcome)
}

// createOperationOutcome creates an OperationOutcome from invalid filters
func createOperationOutcome(invalidFilters []*types.Filter) *fhir.OperationOutcome {
	outcome := &fhir.OperationOutcome{
		Issue: make([]fhir.OperationOutcomeIssue, 0),
	}

	for _, filter := range invalidFilters {
		issue := fhir.OperationOutcomeIssue{
			Severity: fhir.IssueSeverityError,
		}

		switch filter.ErrorType {
		case "unknown-parameter":
			issue.Code = fhir.IssueTypeNotSupported
			diagnostics := fmt.Sprintf("Unknown search parameter '%s'", filter.Code)
			issue.Diagnostics = &diagnostics
			issue.Expression = []string{filter.Code}

		case "unsupported-modifier":
			issue.Code = fhir.IssueTypeNotSupported
			diagnostics := fmt.Sprintf("Search modifier '%s' is not supported for parameter '%s'", filter.Modifier, filter.Code)
			issue.Diagnostics = &diagnostics
			issue.Expression = []string{fmt.Sprintf("%s:%s", filter.Code, filter.Modifier)}

		default:
			issue.Code = fhir.IssueTypeInvalid
			diagnostics := fmt.Sprintf("Invalid parameter '%s'", filter.Code)
			issue.Diagnostics = &diagnostics
			issue.Expression = []string{filter.Code}
		}

		outcome.Issue = append(outcome.Issue, issue)
	}

	return outcome
}

// Example usage:
// GET /fhir/Observation?code:in=http://loinc.org|8480-6
// GET /fhir/Patient?gender=male
// GET /fhir/Condition?code:text=headache
