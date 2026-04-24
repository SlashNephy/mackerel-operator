package containeragent

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceFromObject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		object  client.Object
		want    SourceInput
		wantErr string
	}{
		{
			name:    "nil object",
			object:  nil,
			wantErr: "nil object",
		},
		{
			name:    "typed nil deployment",
			object:  (*appsv1.Deployment)(nil),
			wantErr: "nil object",
		},
		{
			name: "deployment",
			object: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "app"},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								AnnotationInject:           "true",
								AnnotationAPIKeySecretName: "mackerel-api-key",
								AnnotationAPIKeySecretKey:  "apiKey",
								AnnotationImage:            "ghcr.io/mackerelio/mackerel-container-agent:plugins",
								AnnotationConfigSecretName: "mackerel-container-agent-config",
							},
						},
					},
				},
			},
			want: SourceInput{
				Target:           TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled:          true,
				Image:            "ghcr.io/mackerelio/mackerel-container-agent:plugins",
				APIKeySecretName: "mackerel-api-key",
				APIKeySecretKey:  "apiKey",
				ConfigSecretName: "mackerel-container-agent-config",
			},
		},
		{
			name: "statefulset",
			object: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "ops"},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								AnnotationInject:           "false",
								AnnotationAPIKeySecretName: "api-key",
								AnnotationAPIKeySecretKey:  "token",
							},
						},
					},
				},
			},
			want: SourceInput{
				Target:           TargetRef{Kind: "StatefulSet", Namespace: "ops", Name: "db"},
				Enabled:          false,
				APIKeySecretName: "api-key",
				APIKeySecretKey:  "token",
			},
		},
		{
			name: "daemonset",
			object: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "node-agent", Namespace: "infra"},
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								AnnotationInject: "true",
								AnnotationImage:  "example.com/agent:latest",
							},
						},
					},
				},
			},
			want: SourceInput{
				Target:  TargetRef{Kind: "DaemonSet", Namespace: "infra", Name: "node-agent"},
				Enabled: true,
				Image:   "example.com/agent:latest",
			},
		},
		{
			name: "unsupported object kind",
			object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "ignored", Namespace: "app"},
			},
			wantErr: "unsupported object type",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := SourceFromObject(tt.object)
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
