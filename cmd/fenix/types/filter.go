package types

// Filter represents the basic filter input and validation results
type Filter struct {
	Code      string // The search parameter code (e.g., "gender", "status")
	Modifier  string // The modifier (e.g., "exact", "contains")
	Value     string // The value to filter on
	IsValid   bool   // Whether the filter is valid
	ErrorType string // Type of error if invalid (e.g., "unknown-parameter", "invalid-modifier")
}
