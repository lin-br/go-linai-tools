package configs

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v4"
)

const propertiesFilePath = "./internal/configs/configs.yaml"

func isValid(a *string) bool {
	if a == nil || *a == "" {
		return false
	}
	return true
}

// Provider constants.
const (
	ProviderOpenRouter = "openrouter"
	ProviderAnthropic  = "anthropic"
	ProviderBedrock    = "bedrock"
)

type Models struct {
	Default *string `yaml:"default"`
	Pro     *string `yaml:"pro"`
	Free    *string `yaml:"free"`
}

func (m *Models) Get() *string {
	if isValid(m.Default) {
		return m.Default
	}
	if isValid(m.Pro) {
		return m.Pro
	}
	if isValid(m.Free) {
		return m.Free
	}
	return nil
}

type Config struct {
	Provider         string  `yaml:"provider"`
	OpenRouterApiKey *string `yaml:"openrouter_api_key"`
	AnthropicApiKey  *string `yaml:"anthropic_api_key"`
	BedrockRegion    *string `yaml:"bedrock_region"`
	Models           *Models `yaml:"models"`
}

func LoadConfigs() (*Config, error) {
	file, err := os.ReadFile(propertiesFilePath)
	if err != nil {
		return nil, err
	}

	expandedContent := os.ExpandEnv(string(file))

	properties := &Config{}

	if err := yaml.Unmarshal([]byte(expandedContent), properties); err != nil {
		return nil, err
	}

	if properties.Provider == "" {
		properties.Provider = ProviderOpenRouter
	}

	switch properties.Provider {
	case ProviderOpenRouter:
		if !isValid(properties.OpenRouterApiKey) {
			return nil, fmt.Errorf("openrouter_api_key is required when provider is %s", ProviderOpenRouter)
		}
	case ProviderAnthropic, ProviderBedrock:
		// Provider-specific credentials are optional when the provider is not active.
	default:
		return nil, fmt.Errorf("unknown provider: %s", properties.Provider)
	}

	return properties, nil
}
