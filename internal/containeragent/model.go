package containeragent

type TargetRef struct {
	Kind      string
	Namespace string
	Name      string
}

type SourceInput struct {
	Target           TargetRef
	Enabled          bool
	Image            string
	APIKeySecretName string
	APIKeySecretKey  string
	ConfigSecretName string
}

type Config struct {
	Target           TargetRef
	Enabled          bool
	Image            string
	APIKeySecretName string
	APIKeySecretKey  string
}
