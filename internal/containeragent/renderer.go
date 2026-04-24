package containeragent

import corev1 "k8s.io/api/core/v1"

const managedContainerName = "mackerel-container-agent"

type ManagedPodSpec struct {
	Containers []corev1.Container
}

func RenderManagedPodSpec(cfg Config) ManagedPodSpec {
	if !cfg.Enabled {
		return ManagedPodSpec{}
	}

	return ManagedPodSpec{
		Containers: []corev1.Container{
			{
				Name:  managedContainerName,
				Image: cfg.Image,
				Env: []corev1.EnvVar{
					{
						Name: "MACKEREL_APIKEY",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: cfg.APIKeySecretName},
								Key:                  cfg.APIKeySecretKey,
							},
						},
					},
				},
			},
		},
	}
}
