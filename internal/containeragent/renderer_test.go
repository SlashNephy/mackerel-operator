package containeragent

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderManagedPodSpec(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Target:           TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
		Enabled:          true,
		Image:            "ghcr.io/mackerelio/mackerel-container-agent:plugins",
		APIKeySecretName: "mackerel-api-key",
		APIKeySecretKey:  "apiKey",
	}

	got := RenderManagedPodSpec(cfg)

	require.Len(t, got.Containers, 1)
	assert.Equal(t, managedContainerName, got.Containers[0].Name)
	assert.Equal(t, cfg.Image, got.Containers[0].Image)
	require.Len(t, got.Containers[0].Env, 1)
	assert.Equal(t, corev1.EnvVar{
		Name: "MACKEREL_APIKEY",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "mackerel-api-key"},
				Key:                  "apiKey",
			},
		},
	}, got.Containers[0].Env[0])
}
