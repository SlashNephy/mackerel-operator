# Container Agent Sidecar Injection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add annotation-driven `mackerel-container-agent` sidecar injection for `Deployment`, `StatefulSet`, and `DaemonSet`.

**Architecture:** Keep the public API annotation-based, but route reconciliation through internal `source`, `resolver`, `renderer`, and `applier` layers so the same path can accept a future CRD source. Use one controller-runtime reconciler for pod-template workloads and patch only operator-managed fragments.

**Tech Stack:** Go, controller-runtime, Kubernetes apps/v1 workloads, fake client tests, envtest, Kubebuilder RBAC generation.

---

## File Structure

Create or modify these files during implementation:

- `cmd/main.go`: register the new sidecar injection reconciler.
- `internal/controller/containeragent_controller.go`: new reconciler that watches pod-template workloads and applies injection.
- `internal/controller/containeragent_controller_test.go`: envtest or fake-client level reconciler tests for workload resources.
- `internal/controller/containeragent_reconcile_test.go`: table-driven reconcile tests for create/update/remove cases.
- `internal/containeragent/model.go`: internal config, target, and ownership model that does not depend on annotations directly.
- `internal/containeragent/source.go`: reads pod template annotations from supported workload kinds.
- `internal/containeragent/source_test.go`: source extraction tests for each workload kind.
- `internal/containeragent/resolver.go`: normalize source input into the internal config with defaults and validation.
- `internal/containeragent/resolver_test.go`: resolver validation/defaulting tests.
- `internal/containeragent/renderer.go`: render the managed sidecar, env, volume, and mount fragments.
- `internal/containeragent/renderer_test.go`: renderer tests that verify exact managed fragments.
- `internal/containeragent/applier.go`: merge/remove managed fragments on pod templates while preserving user-managed fields.
- `internal/containeragent/applier_test.go`: merge and removal behavior tests.
- `config/rbac/role.yaml`: generated RBAC for `deployments`, `statefulsets`, and `daemonsets`.
- `config/manager/manager.yaml`: generated deployment with updated RBAC references if needed.
- `README.md`: document annotations, defaults, and supported workloads.

## Task 1: Define Internal Container Agent Model And Annotation Source

**Files:**
- Create: `internal/containeragent/model.go`
- Create: `internal/containeragent/source.go`
- Create: `internal/containeragent/source_test.go`

- [ ] **Step 1: Write the failing source tests**

Create `internal/containeragent/source_test.go` with table-driven tests that cover `Deployment`, `StatefulSet`, and `DaemonSet` extraction:

```go
func TestSourceFromObject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		object  client.Object
		want    SourceInput
		wantErr string
	}{
		{
			name: "deployment with injection annotation",
			object: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "app"},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								AnnotationInject:             "true",
								AnnotationAPIKeySecretName:   "mackerel-api-key",
								AnnotationAPIKeySecretKey:    "apiKey",
							},
						},
					},
				},
			},
			want: SourceInput{
				Target: TargetRef{Kind: "Deployment", Namespace: "app", Name: "api"},
				Enabled: true,
				APIKeySecretName: "mackerel-api-key",
				APIKeySecretKey:  "apiKey",
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
```

- [ ] **Step 2: Run the source test to verify it fails**

Run:

```bash
mise exec -- go test ./internal/containeragent -run TestSourceFromObject
```

Expected: FAIL because `SourceInput`, `TargetRef`, annotation constants, and `SourceFromObject` do not exist yet.

- [ ] **Step 3: Add the internal model**

Create `internal/containeragent/model.go` with the initial types:

```go
package containeragent

type TargetRef struct {
	Kind      string
	Namespace string
	Name      string
}

type SourceInput struct {
	Target           TargetRef
	Enabled          bool
	Image            string
	APIKeySecretName string
	APIKeySecretKey  string
	ConfigSecretName string
}

type Config struct {
	Target           TargetRef
	Enabled          bool
	Image            string
	APIKeySecretName string
	APIKeySecretKey  string
	ConfigSecretName string
}
```

- [ ] **Step 4: Implement the annotation source**

Create `internal/containeragent/source.go` with:

```go
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
	switch o := obj.(type) {
	case *appsv1.Deployment:
		return sourceFromTemplate("Deployment", o.Namespace, o.Name, o.Spec.Template.Annotations), nil
	case *appsv1.StatefulSet:
		return sourceFromTemplate("StatefulSet", o.Namespace, o.Name, o.Spec.Template.Annotations), nil
	case *appsv1.DaemonSet:
		return sourceFromTemplate("DaemonSet", o.Namespace, o.Name, o.Spec.Template.Annotations), nil
	default:
		return SourceInput{}, fmt.Errorf("unsupported object type %T", obj)
	}
}

func sourceFromTemplate(kind, namespace, name string, annotations map[string]string) SourceInput {
	return SourceInput{
		Target: TargetRef{Kind: kind, Namespace: namespace, Name: name},
		Enabled: annotations[AnnotationInject] == "true",
		Image: annotations[AnnotationImage],
		APIKeySecretName: annotations[AnnotationAPIKeySecretName],
		APIKeySecretKey: annotations[AnnotationAPIKeySecretKey],
		ConfigSecretName: annotations[AnnotationConfigSecretName],
	}
}
```

- [ ] **Step 5: Run the source test to verify it passes**

Run:

```bash
mise exec -- go test ./internal/containeragent -run TestSourceFromObject
```

Expected: PASS.

- [ ] **Step 6: Commit the source slice**

Run:

```bash
git add internal/containeragent/model.go internal/containeragent/source.go internal/containeragent/source_test.go
git commit -m "feat: add container agent annotation source"
```

Expected: one commit with only the new internal model and source extraction tests.

## Task 2: Add Resolver And Renderer For The Managed Sidecar

**Files:**
- Create: `internal/containeragent/resolver.go`
- Create: `internal/containeragent/resolver_test.go`
- Create: `internal/containeragent/renderer.go`
- Create: `internal/containeragent/renderer_test.go`
- Modify: `internal/containeragent/model.go`

- [ ] **Step 1: Write the failing resolver and renderer tests**

Create `internal/containeragent/resolver_test.go` and `internal/containeragent/renderer_test.go` with these core cases:

```go
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
```

```go
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
	assert.Equal(t, "MACKEREL_APIKEY", got.Containers[0].Env[0].Name)
	assert.Equal(t, "mackerel-api-key", got.Containers[0].Env[0].ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "apiKey", got.Containers[0].Env[0].ValueFrom.SecretKeyRef.Key)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:

```bash
mise exec -- go test ./internal/containeragent -run 'TestResolveConfig|TestRenderManagedPodSpec'
```

Expected: FAIL because `ResolveConfig`, `RenderManagedPodSpec`, defaults, and managed names do not exist yet.

- [ ] **Step 3: Implement the resolver**

Create `internal/containeragent/resolver.go` with:

```go
package containeragent

import "fmt"

const (
	defaultImage            = "ghcr.io/mackerelio/mackerel-container-agent:plugins"
	defaultAPIKeySecretName = "mackerel-api-key"
	defaultAPIKeySecretKey  = "apiKey"
)

func ResolveConfig(input SourceInput) (Config, error) {
	cfg := Config{
		Target:           input.Target,
		Enabled:          input.Enabled,
		Image:            defaultString(input.Image, defaultImage),
		APIKeySecretName: defaultString(input.APIKeySecretName, defaultAPIKeySecretName),
		APIKeySecretKey:  defaultString(input.APIKeySecretKey, defaultAPIKeySecretKey),
		ConfigSecretName: input.ConfigSecretName,
	}

	if !cfg.Enabled {
		return cfg, nil
	}
	if cfg.APIKeySecretName == "" {
		return Config{}, fmt.Errorf("api key secret name is required when injection is enabled")
	}
	if cfg.APIKeySecretKey == "" {
		return Config{}, fmt.Errorf("api key secret key is required when injection is enabled")
	}

	return cfg, nil
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
```

- [ ] **Step 4: Implement the renderer**

Create `internal/containeragent/renderer.go` with:

```go
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
```

- [ ] **Step 5: Run the tests to verify they pass**

Run:

```bash
mise exec -- go test ./internal/containeragent -run 'TestResolveConfig|TestRenderManagedPodSpec'
```

Expected: PASS.

- [ ] **Step 6: Commit the resolver and renderer**

Run:

```bash
git add internal/containeragent/model.go internal/containeragent/resolver.go internal/containeragent/resolver_test.go internal/containeragent/renderer.go internal/containeragent/renderer_test.go
git commit -m "feat: render managed container agent sidecar"
```

Expected: one commit containing defaults, validation, and managed sidecar rendering.

## Task 3: Add Pod Template Applier With Safe Merge And Removal

**Files:**
- Create: `internal/containeragent/applier.go`
- Create: `internal/containeragent/applier_test.go`

- [ ] **Step 1: Write the failing applier tests**

Create `internal/containeragent/applier_test.go` with focused tests:

```go
func TestApplyManagedPodSpecAddsSidecar(t *testing.T) {
	t.Parallel()

	template := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "example.com/app:latest"}},
		},
	}
	managed := ManagedPodSpec{
		Containers: []corev1.Container{{Name: managedContainerName, Image: "agent:latest"}},
	}

	changed := ApplyManagedPodSpec(&template, managed)

	assert.True(t, changed)
	require.Len(t, template.Spec.Containers, 2)
	assert.Equal(t, "app", template.Spec.Containers[0].Name)
	assert.Equal(t, managedContainerName, template.Spec.Containers[1].Name)
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
```

- [ ] **Step 2: Run the applier test to verify it fails**

Run:

```bash
mise exec -- go test ./internal/containeragent -run 'TestApplyManagedPodSpec'
```

Expected: FAIL because `ApplyManagedPodSpec` does not exist yet.

- [ ] **Step 3: Implement the applier**

Create `internal/containeragent/applier.go` with:

```go
package containeragent

import corev1 "k8s.io/api/core/v1"

func ApplyManagedPodSpec(template *corev1.PodTemplateSpec, managed ManagedPodSpec) bool {
	changed := false
	containers := make([]corev1.Container, 0, len(template.Spec.Containers))
	foundManaged := false

	for _, container := range template.Spec.Containers {
		if container.Name != managedContainerName {
			containers = append(containers, container)
			continue
		}

		foundManaged = true
		if len(managed.Containers) == 0 {
			changed = true
			continue
		}

		if container.Image != managed.Containers[0].Image || !envEqual(container.Env, managed.Containers[0].Env) {
			changed = true
			containers = append(containers, managed.Containers[0])
			continue
		}

		containers = append(containers, container)
	}

	if len(managed.Containers) > 0 && !foundManaged {
		containers = append(containers, managed.Containers[0])
		changed = true
	}

	template.Spec.Containers = containers
	return changed
}

func envEqual(left, right []corev1.EnvVar) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Name != right[i].Name {
			return false
		}
		if left[i].Value != right[i].Value {
			return false
		}
		if (left[i].ValueFrom == nil) != (right[i].ValueFrom == nil) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run the applier tests to verify they pass**

Run:

```bash
mise exec -- go test ./internal/containeragent -run 'TestApplyManagedPodSpec'
```

Expected: PASS.

- [ ] **Step 5: Commit the applier**

Run:

```bash
git add internal/containeragent/applier.go internal/containeragent/applier_test.go
git commit -m "feat: apply managed container agent pod fragments"
```

Expected: one commit containing only merge/remove logic and tests.

## Task 4: Add The Workload Reconciler And Watches

**Files:**
- Create: `internal/controller/containeragent_controller.go`
- Create: `internal/controller/containeragent_reconcile_test.go`
- Modify: `cmd/main.go`

- [ ] **Step 1: Write the failing reconcile test**

Create `internal/controller/containeragent_reconcile_test.go` with a fake-client test for `Deployment`:

```go
func TestContainerAgentReconciler_ReconcileDeploymentAddsSidecar(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "app"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						containeragent.AnnotationInject: "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "example.com/app:latest"}},
				},
			},
		},
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deployment).Build()
	reconciler := &ContainerAgentReconciler{Client: k8sClient}

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "api", Namespace: "app"},
	})

	require.NoError(t, err)

	got := &appsv1.Deployment{}
	require.NoError(t, k8sClient.Get(context.Background(), client.ObjectKeyFromObject(deployment), got))
	require.Len(t, got.Spec.Template.Spec.Containers, 2)
	assert.Equal(t, "mackerel-container-agent", got.Spec.Template.Spec.Containers[1].Name)
}
```

- [ ] **Step 2: Run the reconcile test to verify it fails**

Run:

```bash
mise exec -- go test ./internal/controller -run TestContainerAgentReconciler_ReconcileDeploymentAddsSidecar
```

Expected: FAIL because `ContainerAgentReconciler` does not exist yet.

- [ ] **Step 3: Implement the reconciler**

Create `internal/controller/containeragent_controller.go` with:

```go
package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SlashNephy/mackerel-operator/internal/containeragent"
)

type ContainerAgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ContainerAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, deployment); err == nil {
		return r.reconcileDeployment(ctx, deployment)
	}

	statefulSet := &appsv1.StatefulSet{}
	if err := r.Get(ctx, req.NamespacedName, statefulSet); err == nil {
		return r.reconcileStatefulSet(ctx, statefulSet)
	}

	daemonSet := &appsv1.DaemonSet{}
	if err := r.Get(ctx, req.NamespacedName, daemonSet); err == nil {
		return r.reconcileDaemonSet(ctx, daemonSet)
	}

	return ctrl.Result{}, nil
}

func (r *ContainerAgentReconciler) reconcileDeployment(ctx context.Context, deployment *appsv1.Deployment) (ctrl.Result, error) {
	input, err := containeragent.SourceFromObject(deployment)
	if err != nil {
		return ctrl.Result{}, err
	}
	cfg, err := containeragent.ResolveConfig(input)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !containeragent.ApplyManagedPodSpec(&deployment.Spec.Template, containeragent.RenderManagedPodSpec(cfg)) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Update(ctx, deployment)
}
```

Add equivalent `reconcileStatefulSet` and `reconcileDaemonSet` functions that operate on their pod templates.

Add `SetupWithManager`:

```go
func (r *ContainerAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Watches(&appsv1.StatefulSet{}, &handler.EnqueueRequestForObject{}).
		Watches(&appsv1.DaemonSet{}, &handler.EnqueueRequestForObject{}).
		Named("containeragent").
		Complete(r)
}
```

- [ ] **Step 4: Register the reconciler in `cmd/main.go`**

Add after `ExternalMonitorReconciler` setup:

```go
	if err := (&controller.ContainerAgentReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "ContainerAgent")
		os.Exit(1)
	}
```

- [ ] **Step 5: Run the targeted controller test to verify it passes**

Run:

```bash
mise exec -- go test ./internal/controller -run TestContainerAgentReconciler_ReconcileDeploymentAddsSidecar
```

Expected: PASS.

- [ ] **Step 6: Commit the reconciler**

Run:

```bash
git add internal/controller/containeragent_controller.go internal/controller/containeragent_reconcile_test.go cmd/main.go
git commit -m "feat: add container agent reconciler"
```

Expected: one commit containing the new workload controller and manager registration.

## Task 5: Add RBAC, Documentation, And Final Verification

**Files:**
- Modify: `config/rbac/role.yaml`
- Modify: `README.md`

- [ ] **Step 1: Add the failing documentation and RBAC expectation**

Before editing files, capture the missing behavior with this checklist:

```text
- manager role does not allow get/list/watch/update on deployments, statefulsets, daemonsets
- README does not document injection annotations
```

Expected: both items are currently true.

- [ ] **Step 2: Add RBAC markers and regenerate manifests**

In `internal/controller/containeragent_controller.go`, add:

```go
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;update;patch
```

Run:

```bash
mise exec -- make manifests
```

Expected: `config/rbac/role.yaml` includes apps workload permissions.

- [ ] **Step 3: Document the feature in `README.md`**

Add a section like:

```md
## Injecting mackerel-container-agent

The operator can inject `mackerel-container-agent` into supported workloads by pod template annotation.

Supported workloads:

- `Deployment`
- `StatefulSet`
- `DaemonSet`

Example:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: app
spec:
  template:
    metadata:
      annotations:
        mackerel.starry.blue/inject-container-agent: "true"
        mackerel.starry.blue/container-agent-api-key-secret-name: "mackerel-api-key"
        mackerel.starry.blue/container-agent-api-key-secret-key: "apiKey"
```
```

- [ ] **Step 4: Run verification**

Run:

```bash
mise exec -- go test ./internal/containeragent ./internal/controller
mise exec -- make lint
```

Expected: PASS. If `go test ./internal/controller` hits the known local envtest socket restriction, record that limitation and still require the targeted fake-client tests to pass.

- [ ] **Step 5: Commit docs and generated RBAC**

Run:

```bash
git add config/rbac/role.yaml README.md internal/controller/containeragent_controller.go
git commit -m "docs: document container agent injection"
```

Expected: one commit with RBAC generation and docs updates.

## Self-Review

- Spec coverage: supported workloads, annotation API, source/resolver/renderer/applier layering, managed-field-only reconciliation, and docs are all mapped to tasks above. The future CRD itself is intentionally out of scope, matching the spec.
- Placeholder scan: no `TODO`, `TBD`, or implicit "write tests later" steps remain.
- Type consistency: annotation keys, `TargetRef`, `SourceInput`, `Config`, `ManagedPodSpec`, and `ContainerAgentReconciler` use the same names across tasks.
