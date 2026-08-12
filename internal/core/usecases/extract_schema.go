package usecases

// ExtractionResult is the predefined Phase 1 extraction schema. It captures a
// summary, named entities, action items, dates, and monetary amounts from
// free-form text. The tool schema is generated from this struct via
// tools.SchemaFromStruct at ExtractUseCase construction time.
type ExtractionResult struct {
	Summary     string   `json:"summary"`
	Entities    []Entity `json:"entities"`
	ActionItems []string `json:"action_items"`
	Dates       []string `json:"dates"`
	Amounts     []string `json:"amounts"`
}

// Entity is a single named entity extracted from the input text. Type is a
// free-form label (e.g., "person", "organization", "location").
type Entity struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
