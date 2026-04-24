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
			name: "rejects missing secret name when key override is set",
			input: SourceInput{
				Target:          TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:         true,
				APIKeySecretKey: "custom",
			},
			wantErr: "api key secret name",
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
