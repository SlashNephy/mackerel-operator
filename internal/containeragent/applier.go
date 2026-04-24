package containeragent

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
)

func ApplyManagedPodSpec(template *corev1.PodTemplateSpec, managed ManagedPodSpec) bool {
	if template == nil {
		return false
	}

	if len(managed.Containers) == 0 {
		changed := false
		containers := make([]corev1.Container, 0, len(template.Spec.Containers))
		for _, container := range template.Spec.Containers {
			if container.Name == managedContainerName {
				changed = true
				continue
			}
			containers = append(containers, container)
		}

		if changed {
			template.Spec.Containers = containers
		}
		return changed
	}

	desired := managed.Containers[0]
	changed := false
	containers := make([]corev1.Container, 0, len(template.Spec.Containers)+1)
	inserted := false

	for _, container := range template.Spec.Containers {
		if container.Name != managedContainerName {
			containers = append(containers, container)
			continue
		}

		if inserted {
			changed = true
			continue
		}

		inserted = true
		if reflect.DeepEqual(container, desired) {
			containers = append(containers, container)
			continue
		}

		containers = append(containers, desired)
		changed = true
	}

	if !inserted {
		containers = append(containers, desired)
		changed = true
	}

	if changed {
		template.Spec.Containers = containers
	}

	return changed
}
