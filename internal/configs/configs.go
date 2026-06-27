package configs

import (
	"os"

	"go.yaml.in/yaml/v4"
)

const propertiesFilePath = "./internal/configs/configs.yaml"

type Config struct {
	OpenRouterApiKey *string `yaml:"openrouter_api_key"`
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
