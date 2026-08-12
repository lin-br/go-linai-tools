package domain

// CodePlan is the structured output of the spec-to-code use case. It captures
// the high-level summary, the target language hint, and the list of files the
// model proposes to create. It is a plain data struct with no behavior.
type CodePlan struct {
	Summary   string     `json:"summary"`
	Language  string     `json:"language"`
	Files     []FilePlan `json:"files"`
}

// FilePlan describes a single file to create: its path, a short description,
// the types it defines, and the functions it declares. Types and Functions use
// omitempty so a file with only functions omits the types key in JSON.
type FilePlan struct {
	Path        string     `json:"path"`
	Description string     `json:"description"`
	Types       []TypeDecl `json:"types,omitempty"`
	Functions   []FuncDecl `json:"functions,omitempty"`
}

// TypeDecl describes a type (struct, interface, alias) to define in a file.
// A TypeDecl with no fields (e.g., an interface or empty struct) omits the
// fields key in JSON.
type TypeDecl struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Fields      []FieldDecl  `json:"fields,omitempty"`
}

// FieldDecl describes a single field of a TypeDecl: its name, Go type, and an
// optional description.
type FieldDecl struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// FuncDecl describes a function to implement — its name, full signature, and an
// optional description. The Signature carries only the function signature (name,
// parameters, return types), not a body. There is no field for implementation
// code.
type FuncDecl struct {
	Name        string `json:"name"`
	Signature   string `json:"signature"`
	Description string `json:"description,omitempty"`
}
