# Purpose

TBD

# Requirements

## Requirement: Provider selector in config

The system SHALL support a `provider` field in `configs.yaml` that selects the active AI provider. Valid values SHALL be `openrouter` (default), `anthropic`, and `bedrock`. An unknown value SHALL produce a clear error at config load time.

### Scenario: Default provider

- **WHEN** `configs.yaml` does not specify a `provider` field
- **THEN** the system SHALL default to `openrouter`

### Scenario: Unknown provider

- **WHEN** `configs.yaml` specifies `provider: unknown_provider`
- **THEN** `LoadConfigs` SHALL return an error indicating the unknown provider

## Requirement: Per-provider credentials

The system SHALL support per-provider credential fields in `configs.yaml`. For OpenRouter: `openrouter_api_key`. Future providers (`anthropic_api_key`, `bedrock_region`) SHALL be optional and ignored when not the active provider.

### Scenario: OpenRouter credentials

- **WHEN** `provider: openrouter` is set
- **THEN** `LoadConfigs` SHALL require `openrouter_api_key` to be non-empty, and SHALL return an error if it is missing

### Scenario: Inactive provider credentials optional

- **WHEN** `provider: openrouter` is set and `anthropic_api_key` is empty
- **THEN** `LoadConfigs` SHALL NOT return an error for the missing `anthropic_api_key`

## Requirement: Models config preserved

The system SHALL preserve the existing `models` config block (`default`, `pro`, `free`) and the `Models.Get()` resolution logic unchanged.

### Scenario: Model resolution

- **WHEN** `Models.Get()` is called and `default` is set
- **THEN** it SHALL return the default model

## Requirement: Config struct exposes provider and credentials

The `Config` struct SHALL include a `Provider string` field and the existing credential fields. The struct SHALL remain serializable from YAML with environment variable expansion via `os.ExpandEnv`.

### Scenario: Config loads from YAML

- **WHEN** `LoadConfigs()` is called with a valid `configs.yaml`
- **THEN** the returned `*Config` SHALL have `Provider`, `OpenRouterApiKey`, and `Models` populated
