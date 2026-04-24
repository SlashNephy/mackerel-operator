package containeragent

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
	}

	if !cfg.Enabled {
		return cfg, nil
	}

	cfg.Image = defaultString(cfg.Image, defaultImage)

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
