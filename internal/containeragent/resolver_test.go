package containeragent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   SourceInput
		want    Config
		wantErr string
	}{
		{
			name: "applies defaults",
			input: SourceInput{
				Target:  TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled: true,
			},
			want: Config{
				Target:           TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:          true,
				Image:            defaultImage,
				APIKeySecretName: defaultAPIKeySecretName,
				APIKeySecretKey:  defaultAPIKeySecretKey,
			},
		},
		{
			name: "disabled preserves raw values without defaulting",
			input: SourceInput{
				Target:           TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:          false,
				Image:            "example.com/agent:latest",
				APIKeySecretName: "custom-secret",
				APIKeySecretKey:  "custom-key",
				ConfigSecretName: "agent-config",
			},
			want: Config{
				Target:           TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:          false,
				Image:            "example.com/agent:latest",
				APIKeySecretName: "custom-secret",
				APIKeySecretKey:  "custom-key",
			},
		},
		{
			name: "disabled preserves empty fields without defaulting",
			input: SourceInput{
				Target:  TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled: false,
			},
			want: Config{
				Target:  TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled: false,
			},
		},
		{
			name: "applies default key for secret name override",
			input: SourceInput{
				Target:           TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:          true,
				APIKeySecretName: "custom-secret",
			},
			want: Config{
				Target:           TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:          true,
				Image:            defaultImage,
				APIKeySecretName: "custom-secret",
				APIKeySecretKey:  defaultAPIKeySecretKey,
			},
		},
		{
			name: "preserves image override and drops config secret name from resolved config",
			input: SourceInput{
				Target:           TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:          true,
				Image:            "example.com/agent:latest",
				ConfigSecretName: "agent-config",
			},
			want: Config{
				Target:           TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:          true,
				Image:            "example.com/agent:latest",
				APIKeySecretName: defaultAPIKeySecretName,
				APIKeySecretKey:  defaultAPIKeySecretKey,
			},
		},
		{
			name: "applies default secret name for key override",
			input: SourceInput{
				Target:          TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:         true,
				APIKeySecretKey: "custom",
			},
			want: Config{
				Target:           TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:          true,
				Image:            defaultImage,
				APIKeySecretName: defaultAPIKeySecretName,
				APIKeySecretKey:  "custom",
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ResolveConfig(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
