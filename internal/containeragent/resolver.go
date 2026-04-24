package containeragent

import "fmt"

const (
	defaultImage            = "ghcr.io/mackerelio/mackerel-container-agent:plugins"
	defaultAPIKeySecretName = "mackerel-api-key"
	defaultAPIKeySecretKey  = "apiKey"
)

func ResolveConfig(input SourceInput) (Config, error) {
	cfg := Config{
		Target:           input.Target,
		Enabled:          input.Enabled,
		Image:            input.Image,
		APIKeySecretName: input.APIKeySecretName,
		APIKeySecretKey:  input.APIKeySecretKey,
		ConfigSecretName: input.ConfigSecretName,
	}

	if !cfg.Enabled {
		return cfg, nil
	}

	cfg.Image = defaultString(cfg.Image, defaultImage)

	if cfg.APIKeySecretKey != "" && cfg.APIKeySecretName == "" {
		return Config{}, fmt.Errorf("api key secret name is required when api key secret key is set")
	}

	if cfg.APIKeySecretName == "" {
		cfg.APIKeySecretName = defaultAPIKeySecretName
	}
	if cfg.APIKeySecretKey == "" {
		cfg.APIKeySecretKey = defaultAPIKeySecretKey
	}

	return cfg, nil
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
