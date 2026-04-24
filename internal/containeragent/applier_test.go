package containeragent

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyManagedPodSpecAddsSidecar(t *testing.T) {
	t.Parallel()

	template := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "example.com/app:latest"},
			},
		},
	}

	changed := ApplyManagedPodSpec(&template, ManagedPodSpec{
		Containers: []corev1.Container{
			{Name: managedContainerName, Image: "agent:latest"},
		},
	})

	assert.True(t, changed)
	require.Len(t, template.Spec.Containers, 2)
	assert.Equal(t, "app", template.Spec.Containers[0].Name)
	assert.Equal(t, managedContainerName, template.Spec.Containers[1].Name)
}

func TestApplyManagedPodSpecMergesManagedFieldsWithoutTouchingUserContainers(t *testing.T) {
	t.Parallel()

	template := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "example.com/app:latest"},
				{
					Name:  managedContainerName,
					Image: "agent:old",
					Env: []corev1.EnvVar{
						{Name: "MACKEREL_APIKEY", Value: "old"},
					},
					Ports: []corev1.ContainerPort{
						{ContainerPort: 4317, Name: "otlp"},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "agent-config", MountPath: "/etc/agent"},
					},
				},
			},
		},
	}

	changed := ApplyManagedPodSpec(&template, ManagedPodSpec{
		Containers: []corev1.Container{
			{
				Name:  managedContainerName,
				Image: "agent:new",
				Env: []corev1.EnvVar{
					{Name: "MACKEREL_APIKEY", Value: "new"},
				},
			},
		},
	})

	assert.True(t, changed)
	require.Len(t, template.Spec.Containers, 2)
	assert.Equal(t, "app", template.Spec.Containers[0].Name)
	assert.Equal(t, "example.com/app:latest", template.Spec.Containers[0].Image)
	assert.Equal(t, managedContainerName, template.Spec.Containers[1].Name)
	assert.Equal(t, "agent:new", template.Spec.Containers[1].Image)
	assert.Equal(t, []corev1.EnvVar{{Name: "MACKEREL_APIKEY", Value: "new"}}, template.Spec.Containers[1].Env)
	assert.Equal(t, []corev1.ContainerPort{{ContainerPort: 4317, Name: "otlp"}}, template.Spec.Containers[1].Ports)
	assert.Equal(t, []corev1.VolumeMount{{Name: "agent-config", MountPath: "/etc/agent"}}, template.Spec.Containers[1].VolumeMounts)
}

func TestApplyManagedPodSpecRemovesManagedSidecarWhenEmpty(t *testing.T) {
	t.Parallel()

	template := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "example.com/app:latest"},
				{Name: managedContainerName, Image: "agent:old"},
			},
		},
	}

	changed := ApplyManagedPodSpec(&template, ManagedPodSpec{})

	assert.True(t, changed)
	require.Len(t, template.Spec.Containers, 1)
	assert.Equal(t, "app", template.Spec.Containers[0].Name)
}

func TestApplyManagedPodSpecDoesNothingWhenAlreadyMatching(t *testing.T) {
	t.Parallel()

	template := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "example.com/app:latest"},
				{Name: managedContainerName, Image: "agent:latest"},
			},
		},
	}

	changed := ApplyManagedPodSpec(&template, ManagedPodSpec{
		Containers: []corev1.Container{
			{Name: managedContainerName, Image: "agent:latest"},
		},
	})

	assert.False(t, changed)
	require.Len(t, template.Spec.Containers, 2)
	assert.Equal(t, "app", template.Spec.Containers[0].Name)
	assert.Equal(t, managedContainerName, template.Spec.Containers[1].Name)
}
