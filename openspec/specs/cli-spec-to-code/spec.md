# Purpose

Defines the `cmd/spec-to-code` CLI and `SpecToCodeUseCase` that turn a feature description into a structured `CodePlan` (files, types, function signatures) via `tools.Extract[CodePlan]`, with no implementation code in the output. Extends `SchemaFromStruct` (from MP3) to recurse into nested structs and generate array item schemas for struct slices.

# Requirements

### Requirement: CodePlan domain aggregate

The system SHALL define a `CodePlan` struct in `internal/core/domain/codeplan.go` with fields: `Summary string \`json:"summary"\``, `Language string \`json:"language"\``, and `Files []FilePlan \`json:"files"\``. The `CodePlan` aggregate SHALL be a plain data struct with no methods — it represents the structured output of the spec-to-code use case.

#### Scenario: CodePlan with files

- **WHEN** a `CodePlan` is constructed with `Summary: "User auth"`, `Language: "go"`, and one `FilePlan`
- **THEN** the struct SHALL serialize to JSON with `summary`, `language`, and `files` keys

### Requirement: FilePlan struct

The system SHALL define a `FilePlan` struct in `internal/core/domain/codeplan.go` with fields: `Path string \`json:"path"\``, `Description string \`json:"description"\``, `Types []TypeDecl \`json:"types,omitempty"\``, and `Functions []FuncDecl \`json:"functions,omitempty"\``. `Types` and `Functions` SHALL use `omitempty` so a file with only functions omits the `types` key in JSON.

#### Scenario: File with types and functions

- **WHEN** a `FilePlan` has `Path: "internal/core/domain/user.go"`, one `TypeDecl`, and one `FuncDecl`
- **THEN** the JSON output SHALL contain `path`, `types`, and `functions` keys

#### Scenario: File with functions only omits types

- **WHEN** a `FilePlan` has `Path: "main.go"` and one `FuncDecl` but `Types` is nil
- **THEN** the JSON output SHALL omit the `types` key due to `omitempty`

### Requirement: TypeDecl and FieldDecl structs

The system SHALL define a `TypeDecl` struct with fields: `Name string \`json:"name"\``, `Description string \`json:"description,omitempty"\``, and `Fields []FieldDecl \`json:"fields,omitempty"\``. The system SHALL define a `FieldDecl` struct with fields: `Name string \`json:"name"\``, `Type string \`json:"type"\``, and `Description string \`json:"description,omitempty"\``. A `TypeDecl` with no fields (e.g., an interface or empty struct) SHALL omit the `fields` key in JSON.

#### Scenario: Type with fields

- **WHEN** a `TypeDecl` has `Name: "User"` and two `FieldDecl` entries (`ID`/`string`, `Email`/`string`)
- **THEN** the JSON output SHALL contain a `fields` array with two objects each having `name` and `type`

### Requirement: FuncDecl struct

The system SHALL define a `FuncDecl` struct with fields: `Name string \`json:"name"\``, `Signature string \`json:"signature"\``, and `Description string \`json:"description,omitempty"\``. The `Signature` SHALL be a complete function signature string (e.g., `func Login(ctx context.Context, email, password string) (string, error)`) and SHALL NOT contain a function body.

#### Scenario: Function declaration with signature

- **WHEN** a `FuncDecl` has `Name: "Login"` and `Signature: "func Login(ctx context.Context, email, password string) (string, error)"`
- **THEN** the JSON output SHALL contain `name` and `signature` keys, and no `body` key

### Requirement: SchemaFromStruct recursion for nested structs

The system SHALL extend `SchemaFromStruct` (defined in MP3 `internal/core/tools/schema.go`) to recurse into struct-valued fields. When a field's Go type is a struct (or pointer to a struct), the generated property SHALL be an inline JSON Schema object (`{type: "object", properties: {...}, required: [...]}`) produced by recursively calling the schema generator on that field's type. A cycle-detection set SHALL prevent infinite recursion on self-referential structs; when a cycle is detected, the property SHALL fall back to `{}` (opaque).

#### Scenario: Nested struct produces inline schema

- **WHEN** `SchemaFromStruct` is called with a struct containing a field `Inner Outer \`json:"inner"\`` where `Outer` is a struct with `Name string \`json:"name"\``
- **THEN** the `inner` property SHALL be `{type: "object", properties: {name: {type: "string"}}, required: ["name"]}` — not `{}`

#### Scenario: Pointer to struct unwrapped

- **WHEN** `SchemaFromStruct` encounters a field `Config *Config \`json:"config,omitempty"\`` where `Config` is a struct
- **THEN** the field SHALL be treated as the `Config` struct type (pointer unwrapped) and produce an inline schema, excluded from `required` due to `omitempty`

#### Scenario: Self-referential struct does not infinite-loop

- **WHEN** `SchemaFromStruct` encounters a struct `Node` with a field `Next *Node \`json:"next,omitempty"\``
- **THEN** the `next` property SHALL be `{}` (opaque) due to cycle detection, and the generator SHALL terminate without stack overflow

### Requirement: SchemaFromStruct array items for struct slices

The system SHALL extend `SchemaFromStruct` to generate `items` schemas for slices of structs. When a field's Go type is `[]T` where `T` is a struct (or pointer to struct), the generated property SHALL be `{type: "array", items: {<schema for T>}}` with the item schema produced by recursing into `T`. For slices of scalar types (`[]string`, `[]int`, `[]bool`), the `items` SHALL be `{type: "<scalar schema type>"}`.

#### Scenario: Slice of structs produces array with item schema

- **WHEN** `SchemaFromStruct` is called with `CodePlan` which has `Files []FilePlan \`json:"files"\``
- **THEN** the `files` property SHALL be `{type: "array", items: {type: "object", properties: {path: {type: "string"}, ...}, required: ["path"]}}`

#### Scenario: Slice of strings produces array with string items

- **WHEN** `SchemaFromStruct` encounters a field `Tags []string \`json:"tags,omitempty"\``
- **THEN** the property SHALL be `{type: "array", items: {type: "string"}}`

### Requirement: SpecToCodeUseCase

The system SHALL define a `SpecToCodeUseCase` in `internal/core/usecases/spec_to_code.go` that wraps `tools.Extract[CodePlan]`. The use case SHALL have a `Plan(ctx context.Context, input string) (*domain.CodePlan, error)` method. The use case SHALL be constructed via `NewSpecToCodeUseCase(provider outbound.Provider, model string) *SpecToCodeUseCase`, where `model` is the resolved model identifier. The `Plan` method SHALL build a `ToolSchema` for `CodePlan` (via `SchemaFromStruct(&domain.CodePlan{})`), construct the system prompt, and call `tools.Extract[domain.CodePlan]` with the provider, model, system prompt, input, and schema.

#### Scenario: Successful plan generation

- **WHEN** `Plan(ctx, "Add user authentication with login and register")` is called and the provider returns a tool call whose arguments decode into a valid `CodePlan`
- **THEN** the method SHALL return a non-nil `*CodePlan` with populated `Files` and a nil error

#### Scenario: Provider error propagated

- **WHEN** the underlying `Extract[CodePlan]` call returns a non-nil error (e.g., provider HTTP failure)
- **THEN** `Plan` SHALL return a nil `*CodePlan` and the error from `Extract`, without wrapping or transforming it

### Requirement: SpecToCodeUseCase system prompt

The `SpecToCodeUseCase` SHALL use a system prompt that instructs the model to act as a software architect, produce a structured code plan (files, types, functions with signatures), and NOT write implementation code. The system prompt SHALL include the target language hint. The prompt SHALL be defined as a constant in the use case file, not constructed dynamically per call.

#### Scenario: System prompt contains architect instruction

- **WHEN** the use case builds the `ChatRequest` via `Extract`
- **THEN** the system prompt SHALL contain the phrase "software architect" and the phrase "Do not write implementation code"

### Requirement: spec-to-code CLI entry point

The system SHALL provide a CLI entry point at `cmd/spec-to-code/main.go` that reads a feature description from stdin, invokes `SpecToCodeUseCase.Plan`, and writes the result to stdout. The CLI SHALL be runnable via `go run ./cmd/spec-to-code`. The CLI SHALL construct its wiring as: `configs.LoadConfigs()` → `OpenRouterProvider` → `RetryProvider` → `SpecToCodeUseCase`.

#### Scenario: CLI reads stdin and outputs JSON

- **WHEN** the user runs `echo "Add a user authentication system" | go run ./cmd/spec-to-code`
- **THEN** the CLI SHALL read the feature description from stdin, call the use case, and print a JSON `CodePlan` to stdout

#### Scenario: Empty stdin produces error

- **WHEN** stdin is empty (zero bytes)
- **THEN** the CLI SHALL print "no input: provide a feature description via stdin" to stderr and exit with code 1

### Requirement: CLI flags

The CLI SHALL accept the following flags via the `flag` package: `-model string` (override the config default model; empty string means use config default), `-format string` (accepted values `json` and `text`; default `json`), and `-lang string` (target language hint; default `go`). An invalid `-format` value SHALL cause the CLI to print an error to stderr and exit with code 1.

#### Scenario: Default format is JSON

- **WHEN** the user runs the CLI without `-format`
- **THEN** the output SHALL be JSON (`json.MarshalIndent` with 2-space indent)

#### Scenario: Text format produces tree

- **WHEN** the user runs `echo "..." | go run ./cmd/spec-to-code -format text`
- **THEN** the output SHALL be a human-readable ASCII tree showing the plan structure, not JSON

#### Scenario: Invalid format value errors

- **WHEN** the user runs the CLI with `-format yaml`
- **THEN** the CLI SHALL print an error to stderr and exit with code 1

#### Scenario: Model flag overrides config

- **WHEN** the user runs the CLI with `-model anthropic/claude-sonnet-4-20250514`
- **THEN** the use case SHALL use that model instead of the config `DEFAULT_MODEL`

#### Scenario: Lang flag defaults to go

- **WHEN** the user runs the CLI without `-lang`
- **THEN** the system prompt SHALL include "Target language: go"

### Requirement: JSON output format

When `-format json` (or default), the CLI SHALL render the `CodePlan` via `json.MarshalIndent(plan, "", "  ")` and write it to stdout followed by a newline. The output SHALL be valid JSON parseable by `jq`.

#### Scenario: JSON output is indented

- **WHEN** the CLI produces JSON output for a `CodePlan` with one file
- **THEN** the output SHALL be indented with 2 spaces per level and SHALL end with a newline character

### Requirement: Text output format — ASCII tree

When `-format text`, the CLI SHALL render the `CodePlan` as a human-readable ASCII tree to stdout. The tree SHALL display: the plan summary and language as a header, each file path as a top-level entry, `Types:` and `Functions:` sections under each file, type names with their fields indented beneath, and function signatures listed beneath the `Functions:` label. The tree SHALL NOT use emojis. The tree MAY use ASCII indentation (spaces) for hierarchy.

#### Scenario: Text tree renders files and types

- **WHEN** the CLI renders a `CodePlan` with a file `internal/core/domain/user.go` containing a `TypeDecl` named `User` with fields `ID string` and `Email string`
- **THEN** the output SHALL contain the file path, a `Types:` label, the type name `User`, and the field lines `ID: string` and `Email: string` indented beneath the type

#### Scenario: Text tree renders functions

- **WHEN** the CLI renders a `FilePlan` with a `FuncDecl` whose `Signature` is `func Login(ctx context.Context, email, password string) (string, error)`
- **THEN** the output SHALL contain a `Functions:` label and the signature string indented beneath it

### Requirement: Context cancellation via signal.NotifyContext

The CLI SHALL create a context via `signal.NotifyContext(context.Background(), os.Interrupt)` and pass it to `SpecToCodeUseCase.Plan`. When the user presses Ctrl+C (SIGINT), the context SHALL be cancelled, aborting any in-flight provider call or retry backoff. The CLI SHALL exit with code 130 on SIGINT and SHALL NOT print a plan.

#### Scenario: Ctrl+C aborts in-flight request

- **WHEN** the user presses Ctrl+C while the provider call is in flight
- **THEN** the context SHALL be cancelled, the provider call SHALL be abandoned, and the CLI SHALL exit with code 130

### Requirement: Error-to-exit-code mapping

The CLI SHALL map errors from the use case to exit codes as follows: `ErrNoToolCall` → exit 1 with message "model did not return a structured plan" to stderr; `ErrToolNameMismatch` → exit 1 with message "model returned an unexpected tool call" to stderr; `ErrUnmarshalFailed` → exit 1 with message "failed to parse model output: <wrapped detail>" to stderr; provider/network error → exit 1 with message "request failed: <error>" to stderr; `context.Canceled` → exit 130 with no message. All error messages SHALL go to stderr, not stdout.

#### Scenario: ErrNoToolCall exits 1

- **WHEN** the use case returns `ErrNoToolCall`
- **THEN** the CLI SHALL print "model did not return a structured plan" to stderr and exit with code 1

#### Scenario: ErrUnmarshalFailed includes detail

- **WHEN** the use case returns `ErrUnmarshalFailed` wrapping a `json.SyntaxError`
- **THEN** the CLI SHALL print a message to stderr containing "failed to parse model output" and the wrapped error detail, and exit with code 1

#### Scenario: Provider error exits 1

- **WHEN** the use case returns a non-nil error that is not a sentinel or context error
- **THEN** the CLI SHALL print "request failed: <error>" to stderr and exit with code 1

### Requirement: No implementation code in output

The `CodePlan` aggregate and `FuncDecl` struct SHALL NOT include any field for function bodies or implementation code. The `FuncDecl.Signature` field SHALL contain only the function signature (name, parameters, return types), not a body. The system prompt SHALL instruct the model to produce plans, not code.

#### Scenario: FuncDecl has no body field

- **WHEN** a `FuncDecl` is marshaled to JSON
- **THEN** the JSON SHALL contain `name`, `signature`, and optionally `description` keys, and SHALL NOT contain a `body`, `implementation`, or `code` key
