package containeragent

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderManagedPodSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
		want ManagedPodSpec
	}{
		{
			name: "renders managed container",
			cfg: Config{
				Target:           TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:          true,
				Image:            "ghcr.io/mackerelio/mackerel-container-agent:plugins",
				APIKeySecretName: "mackerel-api-key",
				APIKeySecretKey:  "apiKey",
			},
			want: ManagedPodSpec{
				Containers: []corev1.Container{
					{
						Name:  managedContainerName,
						Image: "ghcr.io/mackerelio/mackerel-container-agent:plugins",
						Env: []corev1.EnvVar{
							{
								Name: "MACKEREL_APIKEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: "mackerel-api-key"},
										Key:                  "apiKey",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "disabled returns empty managed pod spec",
			cfg: Config{
				Target:  TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled: false,
			},
			want: ManagedPodSpec{},
		},
		{
			name: "invalid enabled config returns empty managed pod spec",
			cfg: Config{
				Target:  TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled: true,
				Image:   "ghcr.io/mackerelio/mackerel-container-agent:plugins",
			},
			want: ManagedPodSpec{},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := RenderManagedPodSpec(tt.cfg)

			assert.Equal(t, tt.want, got)
			if len(tt.want.Containers) > 0 {
				require.Len(t, got.Containers, len(tt.want.Containers))
			}
		})
	}
}
