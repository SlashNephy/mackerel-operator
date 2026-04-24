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

func TestApplyManagedPodSpecReplacesManagedSidecarWithoutTouchingUserContainers(t *testing.T) {
	t.Parallel()

	template := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "example.com/app:latest"},
				{Name: managedContainerName, Image: "agent:old"},
			},
		},
	}

	changed := ApplyManagedPodSpec(&template, ManagedPodSpec{
		Containers: []corev1.Container{
			{Name: managedContainerName, Image: "agent:new"},
		},
	})

	assert.True(t, changed)
	require.Len(t, template.Spec.Containers, 2)
	assert.Equal(t, "app", template.Spec.Containers[0].Name)
	assert.Equal(t, "example.com/app:latest", template.Spec.Containers[0].Image)
	assert.Equal(t, managedContainerName, template.Spec.Containers[1].Name)
	assert.Equal(t, "agent:new", template.Spec.Containers[1].Image)
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
