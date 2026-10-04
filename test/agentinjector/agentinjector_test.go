/*
Copyright 2026 SlashNephy.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package agentinjector applies the MutatingAdmissionPolicy shipped by the chart
// to an envtest kube-apiserver and checks the specs of the Pods it creates.
package agentinjector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	chartPath          = "../../charts/mackerel-operator"
	mapAPIVersion      = "admissionregistration.k8s.io/v1/MutatingAdmissionPolicy"
	sidecarName        = "mackerel-container-agent"
	tokenVolumeName    = "mackerel-agent-sa-token"
	configVolumeName   = "mackerel-agent-config"
	labelInject        = "agent.mackerel.starry.blue/inject"
	annotationSecret   = "agent.mackerel.starry.blue/apikey-secret-name"
	annotationKey      = "agent.mackerel.starry.blue/apikey-secret-key"
	annotationRoles    = "agent.mackerel.starry.blue/roles"
	annotationConfigCM = "agent.mackerel.starry.blue/config-configmap-name"
	testNamespace      = "agent-injector-test"
)

var k8sClient client.Client

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	manifests, err := renderChart("--api-versions", mapAPIVersion)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	testEnv := &envtest.Environment{}
	// BinaryAssetsDirectory takes precedence over KUBEBUILDER_ASSETS, so only set it as a fallback.
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		testEnv.BinaryAssetsDirectory = newestEnvTestBinaryDir()
	}
	cfg, err := testEnv.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() {
		_ = testEnv.Stop()
	}()

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ctx := context.Background()
	if err := setup(ctx, manifests); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return m.Run()
}

// renderChart renders only the agent injector template with extra helm flags.
func renderChart(extraArgs ...string) ([]byte, error) {
	args := slices.Concat([]string{
		"template", "test", chartPath,
		"--show-only", "templates/agent-injector.yaml",
		"--set", "agentInjector.enabled=true",
		// Fixed so that Renovate bumping the default tag does not break the test.
		"--set", "agentInjector.image.repository=example.com/agent",
		"--set", "agentInjector.image.tag=test",
	}, extraArgs)

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("helm", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("helm template failed: %w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func setup(ctx context.Context, manifests []byte) error {
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifests), 4096)
	for {
		obj := &unstructured.Unstructured{}
		if err := decoder.Decode(&obj.Object); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to decode manifest: %w", err)
		}
		if len(obj.Object) == 0 {
			continue
		}
		if err := k8sClient.Create(ctx, obj); err != nil {
			return fmt.Errorf("failed to create %s %s: %w", obj.GetKind(), obj.GetName(), err)
		}
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
	if err := k8sClient.Create(ctx, ns); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: testNamespace}}
	if err := k8sClient.Create(ctx, sa); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create service account: %w", err)
	}

	return waitForPolicy(ctx)
}

// waitForPolicy recreates a probe Pod until the kube-apiserver has loaded the policy.
func waitForPolicy(ctx context.Context) error {
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		pod := newPod("probe", map[string]string{labelInject: "true"}, map[string]string{annotationSecret: "probe"})
		if err := k8sClient.Create(ctx, pod); err != nil {
			return fmt.Errorf("failed to create probe pod: %w", err)
		}
		injected := findSidecar(pod.Spec.InitContainers) != nil
		if err := k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0)); err != nil {
			return fmt.Errorf("failed to delete probe pod: %w", err)
		}
		if injected {
			return nil
		}
		time.Sleep(time.Second)
	}
	return errors.New("mutating admission policy did not become effective")
}

func newPod(name string, labels, annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   testNamespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{
			// envtest disables the ServiceAccount admission plugin, which fills this in on real clusters.
			ServiceAccountName: "default",
			Containers:         []corev1.Container{{Name: "app", Image: "busybox"}},
		},
	}
}

func findSidecar(containers []corev1.Container) *corev1.Container {
	for i := range containers {
		if containers[i].Name == sidecarName {
			return &containers[i]
		}
	}
	return nil
}

func findEnv(env []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range env {
		if env[i].Name == name {
			return &env[i]
		}
	}
	return nil
}

func volumeNames(volumes []corev1.Volume) []string {
	names := make([]string, 0, len(volumes))
	for _, v := range volumes {
		names = append(names, v.Name)
	}
	return names
}

func containerNames(containers []corev1.Container) []string {
	names := make([]string, 0, len(containers))
	for _, c := range containers {
		names = append(names, c.Name)
	}
	return names
}

func TestInjection(t *testing.T) {
	ctx := context.Background()
	inject := map[string]string{labelInject: "true"}

	tests := []struct {
		name   string
		pod    func() *corev1.Pod
		assert func(t *testing.T, pod *corev1.Pod)
	}{
		{
			name: "injects sidecar with every annotation",
			pod: func() *corev1.Pod {
				pod := newPod("full", inject, map[string]string{
					annotationSecret:   "mackerel-api-key",
					annotationRoles:    "service:role",
					annotationConfigCM: "agent-config",
				})
				pod.Spec.AutomountServiceAccountToken = new(false)
				return pod
			},
			assert: func(t *testing.T, pod *corev1.Pod) {
				sidecar := findSidecar(pod.Spec.InitContainers)
				require.NotNil(t, sidecar)
				require.NotNil(t, sidecar.RestartPolicy)
				assert.Equal(t, corev1.ContainerRestartPolicyAlways, *sidecar.RestartPolicy)
				assert.Equal(t, "example.com/agent:test", sidecar.Image)
				assert.Equal(t, corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				}, sidecar.Resources.Limits)
				assert.Equal(t, &corev1.SecurityContext{
					RunAsNonRoot:             new(true),
					RunAsUser:                new(int64(65532)),
					AllowPrivilegeEscalation: new(false),
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
					SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				}, sidecar.SecurityContext)

				apiKey := findEnv(sidecar.Env, "MACKEREL_APIKEY")
				require.NotNil(t, apiKey)
				require.NotNil(t, apiKey.ValueFrom)
				require.NotNil(t, apiKey.ValueFrom.SecretKeyRef)
				assert.Equal(t, "mackerel-api-key", apiKey.ValueFrom.SecretKeyRef.Name)
				assert.Equal(t, "MACKEREL_APIKEY", apiKey.ValueFrom.SecretKeyRef.Key)
				require.NotNil(t, apiKey.ValueFrom.SecretKeyRef.Optional)
				assert.True(t, *apiKey.ValueFrom.SecretKeyRef.Optional)

				for name, want := range map[string]string{
					"MACKEREL_CONTAINER_PLATFORM":                "kubernetes",
					"MACKEREL_KUBERNETES_KUBELET_READ_ONLY_PORT": "0",
					"MACKEREL_KUBERNETES_KUBELET_INSECURE_TLS":   "true",
					"MACKEREL_ROLES":                             "service:role",
					"MACKEREL_AGENT_CONFIG":                      "/etc/mackerel-agent/mackerel-agent.conf",
				} {
					env := findEnv(sidecar.Env, name)
					if assert.NotNil(t, env, name) {
						assert.Equal(t, want, env.Value, name)
					}
				}
				for name, want := range map[string]string{
					"MACKEREL_KUBERNETES_KUBELET_HOST": "status.hostIP",
					"MACKEREL_KUBERNETES_NAMESPACE":    "metadata.namespace",
					"MACKEREL_KUBERNETES_POD_NAME":     "metadata.name",
				} {
					env := findEnv(sidecar.Env, name)
					require.NotNil(t, env, name)
					require.NotNil(t, env.ValueFrom, name)
					require.NotNil(t, env.ValueFrom.FieldRef, name)
					assert.Equal(t, want, env.ValueFrom.FieldRef.FieldPath, name)
				}

				assert.ElementsMatch(t, []corev1.VolumeMount{
					{Name: tokenVolumeName, MountPath: "/var/run/secrets/kubernetes.io/serviceaccount", ReadOnly: true},
					{Name: configVolumeName, MountPath: "/etc/mackerel-agent", ReadOnly: true},
				}, sidecar.VolumeMounts)
				assert.ElementsMatch(t, []string{tokenVolumeName, configVolumeName}, volumeNames(pod.Spec.Volumes))

				for _, v := range pod.Spec.Volumes {
					switch v.Name {
					case tokenVolumeName:
						require.NotNil(t, v.Projected)
						require.Len(t, v.Projected.Sources, 3)
						require.NotNil(t, v.Projected.Sources[0].ServiceAccountToken)
						assert.Equal(t, int64(3607), *v.Projected.Sources[0].ServiceAccountToken.ExpirationSeconds)
					case configVolumeName:
						require.NotNil(t, v.ConfigMap)
						assert.Equal(t, "agent-config", v.ConfigMap.Name)
					}
				}
			},
		},
		{
			name: "uses the secret key from the annotation",
			pod: func() *corev1.Pod {
				return newPod("secret-key", inject, map[string]string{
					annotationSecret: "other",
					annotationKey:    "apiKey",
				})
			},
			assert: func(t *testing.T, pod *corev1.Pod) {
				sidecar := findSidecar(pod.Spec.InitContainers)
				require.NotNil(t, sidecar)
				apiKey := findEnv(sidecar.Env, "MACKEREL_APIKEY")
				require.NotNil(t, apiKey)
				assert.Equal(t, "other", apiKey.ValueFrom.SecretKeyRef.Name)
				assert.Equal(t, "apiKey", apiKey.ValueFrom.SecretKeyRef.Key)
				assert.Nil(t, findEnv(sidecar.Env, "MACKEREL_ROLES"))
				assert.Nil(t, findEnv(sidecar.Env, "MACKEREL_AGENT_CONFIG"))
				assert.Equal(t, []string{tokenVolumeName}, volumeNames(pod.Spec.Volumes))
			},
		},
		{
			name: "ignores pods without the label",
			pod: func() *corev1.Pod {
				return newPod("no-label", nil, map[string]string{annotationSecret: "mackerel-api-key"})
			},
			assert: func(t *testing.T, pod *corev1.Pod) {
				assert.Empty(t, pod.Spec.InitContainers)
				assert.NotContains(t, volumeNames(pod.Spec.Volumes), tokenVolumeName)
			},
		},
		{
			name: "ignores pods without the secret annotation",
			pod: func() *corev1.Pod {
				return newPod("no-secret", inject, nil)
			},
			assert: func(t *testing.T, pod *corev1.Pod) {
				assert.Empty(t, pod.Spec.InitContainers)
				assert.NotContains(t, volumeNames(pod.Spec.Volumes), tokenVolumeName)
			},
		},
		{
			name: "does not inject twice",
			pod: func() *corev1.Pod {
				pod := newPod("already-injected", inject, map[string]string{annotationSecret: "mackerel-api-key"})
				pod.Spec.InitContainers = []corev1.Container{{Name: sidecarName, Image: "custom"}}
				return pod
			},
			assert: func(t *testing.T, pod *corev1.Pod) {
				require.Len(t, pod.Spec.InitContainers, 1)
				assert.Equal(t, "custom", pod.Spec.InitContainers[0].Image)
			},
		},
		{
			name: "skips pods whose volume names conflict",
			pod: func() *corev1.Pod {
				pod := newPod("volume-conflict", inject, map[string]string{annotationSecret: "mackerel-api-key"})
				pod.Spec.Volumes = []corev1.Volume{{
					Name:         tokenVolumeName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				}}
				return pod
			},
			assert: func(t *testing.T, pod *corev1.Pod) {
				assert.Empty(t, pod.Spec.InitContainers)
				assert.Equal(t, []string{tokenVolumeName}, volumeNames(pod.Spec.Volumes))
			},
		},
		{
			name: "appends to existing init containers and volumes",
			pod: func() *corev1.Pod {
				pod := newPod("existing", inject, map[string]string{annotationSecret: "mackerel-api-key"})
				pod.Spec.InitContainers = []corev1.Container{{Name: "init", Image: "busybox"}}
				pod.Spec.Volumes = []corev1.Volume{{
					Name:         "data",
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				}}
				return pod
			},
			assert: func(t *testing.T, pod *corev1.Pod) {
				assert.Equal(t, []string{"init", sidecarName}, containerNames(pod.Spec.InitContainers))
				assert.Contains(t, volumeNames(pod.Spec.Volumes), "data")
				assert.Contains(t, volumeNames(pod.Spec.Volumes), tokenVolumeName)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := tt.pod()
			require.NoError(t, k8sClient.Create(ctx, pod))
			t.Cleanup(func() {
				_ = k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0))
			})
			tt.assert(t, pod)
		})
	}
}

// TestRestrictedNamespace checks that the injected sidecar does not make a Pod
// violate the restricted Pod Security Standard.
func TestRestrictedNamespace(t *testing.T) {
	ctx := context.Background()
	const namespace = "agent-injector-restricted"

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   namespace,
		Labels: map[string]string{"pod-security.kubernetes.io/enforce": "restricted"},
	}}
	require.NoError(t, k8sClient.Create(ctx, ns))
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: namespace}}
	require.NoError(t, client.IgnoreAlreadyExists(k8sClient.Create(ctx, sa)))

	pod := newPod("restricted",
		map[string]string{labelInject: "true"},
		map[string]string{annotationSecret: "mackerel-api-key"},
	)
	pod.Namespace = namespace
	pod.Spec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot:   new(true),
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
	pod.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
		RunAsUser:                new(int64(1000)),
		AllowPrivilegeEscalation: new(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}

	require.NoError(t, k8sClient.Create(ctx, pod))
	assert.NotNil(t, findSidecar(pod.Spec.InitContainers))
}

func TestRenderRequiresPolicyAPI(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name: "cluster serving the policy API",
			args: []string{"--kube-version", "1.35.0", "--api-versions", mapAPIVersion},
		},
		{
			// helm template and GitOps tools render without a cluster, so only the version is known.
			name: "offline rendering for 1.36",
			args: []string{"--kube-version", "1.36.0"},
		},
		{
			name:    "offline rendering for 1.35",
			args:    []string{"--kube-version", "1.35.0"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := renderChart(tt.args...)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "MutatingAdmissionPolicy")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRenderWithoutResources(t *testing.T) {
	manifests, err := renderChart("--kube-version", "1.36.0", "--set", "agentInjector.resources=null")
	require.NoError(t, err)
	assert.Contains(t, string(manifests), "resources: Object.spec.initContainers.resources{}")
}

// newestEnvTestBinaryDir locates envtest binaries when the test runs without the
// Makefile. It picks the last entry because MutatingAdmissionPolicy needs 1.36+.
func newestEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return ""
	}
	for _, entry := range slices.Backward(entries) {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}
