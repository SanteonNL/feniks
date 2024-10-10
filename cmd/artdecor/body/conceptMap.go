package body

type GetConceptMapListResponse struct {
	Artifact   string `json:"artifact"`
	Current    string `json:"current"`
	Total      string `json:"total"`
	All        string `json:"all"`
	ConceptMap []any  `json:"conceptMap,omitempty"`
}
