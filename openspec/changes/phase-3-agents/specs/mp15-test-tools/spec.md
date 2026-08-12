## ADDED Requirements

### Requirement: Tool interface is defined in the agent package

The system SHALL define an `agent.Tool` interface in `internal/agent/types.go` (or `internal/agent/tool.go`) with methods `Name() string`, `Description() string`, `InputSchema() map[string]any`, and `Execute(ctx context.Context, args json.RawMessage) (string, error)`. All test tools SHALL implement this interface.

#### Scenario: Tool interface is satisfiable

- **WHEN** a type implements the four methods of `agent.Tool`
- **THEN** a compile-time blank identifier assignment `var _ agent.Tool = (*MyTool)(nil)` SHALL succeed

#### Scenario: Execute receives raw JSON arguments

- **WHEN** the loop calls `tool.Execute(ctx, []byte("{\"location\":\"São Paulo\"}"))` on a tool implementation
- **THEN** the tool SHALL parse the arguments and return a string result or error

### Requirement: get_weather tool returns deterministic weather information

The system SHALL provide `internal/agent/tools/weather.go` with a `NewWeather() agent.Tool` constructor. The tool SHALL be named `get_weather`, accept a JSON object with a `location` string field, and return a deterministic string describing weather for known locations. For unknown locations it SHALL return a fallback string without making a real HTTP call.

#### Scenario: Known location returns weather

- **WHEN** `get_weather.Execute(ctx, []byte("{\"location\":\"São Paulo\"}"))` is called
- **THEN** it SHALL return a string containing temperature and condition for São Paulo

#### Scenario: Unknown location returns fallback

- **WHEN** `get_weather.Execute(ctx, []byte("{\"location\":\"Atlantis\"}"))` is called
- **THEN** it SHALL return a fallback message stating the location is not covered

### Requirement: search_wiki tool searches a local wiki directory

The system SHALL provide `internal/agent/tools/wiki.go` with a `NewWikiSearch(dir string) agent.Tool` constructor. The tool SHALL be named `search_wiki`, accept a JSON object with a `query` string field, scan markdown files under `dir`, and return a JSON string containing matching file names and snippets. If `dir` is empty, the constructor SHALL default to `./wiki`.

#### Scenario: Existing wiki file matches query

- **WHEN** `./wiki` contains `notes.md` with the text "agent loop" and `search_wiki` is called with query "agent loop"
- **THEN** the tool SHALL return a result that includes the file path and a matching snippet

#### Scenario: No matches returns empty result

- **WHEN** `search_wiki` is called with a query that does not appear in any file under the configured directory
- **THEN** it SHALL return a string indicating no matches were found

### Requirement: calculate tool evaluates arithmetic expressions

The system SHALL provide `internal/agent/tools/calculate.go` with a `NewCalculate() agent.Tool` constructor. The tool SHALL be named `calculate`, accept a JSON object with an `expression` string field, and evaluate a safe subset of arithmetic expressions (addition, subtraction, multiplication, division, parentheses). Division by zero SHALL return an error.

#### Scenario: Simple expression evaluates correctly

- **WHEN** `calculate.Execute(ctx, []byte("{\"expression\":\"(2 + 3) * 4\"}"))` is called
- **THEN** it SHALL return the string "20"

#### Scenario: Division by zero returns error

- **WHEN** `calculate.Execute(ctx, []byte("{\"expression\":\"10 / 0\"}"))` is called
- **THEN** it SHALL return a non-nil error

#### Scenario: Invalid expression returns error

- **WHEN** `calculate.Execute(ctx, []byte("{\"expression\":\"2 + * 3\"}"))` is called
- **THEN** it SHALL return a non-nil error

### Requirement: Tool input schemas are generated from structs

The system SHALL generate each tool's `InputSchema()` using the existing `tools.SchemaFromStruct` helper or a hand-built `map[string]any` that matches the tool's argument struct. Each schema SHALL declare the required fields (`location`, `query`, `expression`) and the property types.

#### Scenario: Weather tool schema matches expected arguments

- **WHEN** `get_weather.InputSchema()` is called
- **THEN** it SHALL return a schema with `"type": "object"`, a `location` property of type `string`, and `location` listed in `required`

#### Scenario: Wiki tool schema matches expected arguments

- **WHEN** `search_wiki.InputSchema()` is called
- **THEN** it SHALL return a schema with `"type": "object"`, a `query` property of type `string`, and `query` listed in `required`

#### Scenario: Calculate tool schema matches expected arguments

- **WHEN** `calculate.InputSchema()` is called
- **THEN** it SHALL return a schema with `"type": "object"`, an `expression` property of type `string`, and `expression` listed in `required`
