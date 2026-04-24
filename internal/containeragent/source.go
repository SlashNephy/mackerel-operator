package containeragent

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AnnotationInject           = "mackerel.starry.blue/inject-container-agent"
	AnnotationAPIKeySecretName = "mackerel.starry.blue/container-agent-api-key-secret-name"
	AnnotationAPIKeySecretKey  = "mackerel.starry.blue/container-agent-api-key-secret-key"
	AnnotationImage            = "mackerel.starry.blue/container-agent-image"
	AnnotationConfigSecretName = "mackerel.starry.blue/container-agent-config-secret-name"
)

func SourceFromObject(obj client.Object) (SourceInput, error) {
	if obj == nil {
		return SourceInput{}, fmt.Errorf("nil object")
	}

	switch o := obj.(type) {
	case *appsv1.Deployment:
		if o == nil {
			return SourceInput{}, fmt.Errorf("nil object")
		}
		return sourceFromTemplate("Deployment", o.Namespace, o.Name, o.Spec.Template.Annotations), nil
	case *appsv1.StatefulSet:
		if o == nil {
			return SourceInput{}, fmt.Errorf("nil object")
		}
		return sourceFromTemplate("StatefulSet", o.Namespace, o.Name, o.Spec.Template.Annotations), nil
	case *appsv1.DaemonSet:
		if o == nil {
			return SourceInput{}, fmt.Errorf("nil object")
		}
		return sourceFromTemplate("DaemonSet", o.Namespace, o.Name, o.Spec.Template.Annotations), nil
	default:
		return SourceInput{}, fmt.Errorf("unsupported object type %T", obj)
	}
}

func sourceFromTemplate(kind, namespace, name string, annotations map[string]string) SourceInput {
	return SourceInput{
		Target:           TargetRef{Kind: kind, Namespace: namespace, Name: name},
		Enabled:          annotations[AnnotationInject] == "true",
		Image:            annotations[AnnotationImage],
		APIKeySecretName: annotations[AnnotationAPIKeySecretName],
		APIKeySecretKey:  annotations[AnnotationAPIKeySecretKey],
		ConfigSecretName: annotations[AnnotationConfigSecretName],
	}
}
