package configs

import (
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
	OpenRouterApiKey *string `yaml:"openrouter_api_key"`
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
	return properties, nil
}
