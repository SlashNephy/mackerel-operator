package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/SlashNephy/mackerel-operator/internal/ownership"
	operatorstatus "github.com/SlashNephy/mackerel-operator/internal/status"
)

func secretBackedExternalMonitor(finalizers ...string) *mackerelv1alpha1.ExternalMonitor {
	return &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "api-health",
			Namespace:  "default",
			Finalizers: finalizers,
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://example.com/healthz",
			Headers: new([]mackerelv1alpha1.HeaderField{{
				Name: "Authorization",
				ValueFrom: &mackerelv1alpha1.HeaderValueSource{
					SecretKeyRef: &mackerelv1alpha1.SecretKeySelector{Name: "api-credentials", Key: "token"},
				},
			}}),
		},
	}
}

func TestExternalMonitorReconciler_ReconcileResolvesSecretBackedHeader(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newExternalMonitorTestScheme(t)
	cr := secretBackedExternalMonitor()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "api-credentials", Namespace: "default"},
		Data:       map[string][]byte{"token": []byte("Bearer token")},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr, secret).
		WithStatusSubresource(&mackerelv1alpha1.ExternalMonitor{}).
		Build()
	provider := newFakeExternalMonitorProvider()
	reconciler := &ExternalMonitorReconciler{
		Client:     k8sClient,
		Scheme:     scheme,
		Provider:   provider,
		OwnerID:    "prod",
		Policy:     "sync",
		HashLength: 7,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "api-health", Namespace: "default"},
	})

	require.NoError(t, err)
	require.Len(t, provider.created, 1)
	require.NotNil(t, provider.created[0].Headers)
	assert.Equal(t, []monitor.HeaderField{{Name: "Authorization", Value: "Bearer token"}}, *provider.created[0].Headers)

	synced := &mackerelv1alpha1.ExternalMonitor{}
	require.NoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), synced))
	ready := meta.FindStatusCondition(synced.Status.Conditions, operatorstatus.ConditionReady)
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionTrue, ready.Status)
}

func TestExternalMonitorReconciler_ReconcileBlocksOnUnresolvableHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		secret *corev1.Secret
	}{
		{
			name: "secret is missing",
		},
		{
			name: "secret lacks the referenced key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "api-credentials", Namespace: "default"},
				Data:       map[string][]byte{"other": []byte("Bearer token")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			scheme := newExternalMonitorTestScheme(t)
			cr := secretBackedExternalMonitor()
			objects := []client.Object{cr}
			if tt.secret != nil {
				objects = append(objects, tt.secret)
			}
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				WithStatusSubresource(&mackerelv1alpha1.ExternalMonitor{}).
				Build()
			provider := newFakeExternalMonitorProvider()
			reconciler := &ExternalMonitorReconciler{
				Client:     k8sClient,
				Scheme:     scheme,
				Provider:   provider,
				OwnerID:    "prod",
				Policy:     "sync",
				HashLength: 7,
			}

			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "api-health", Namespace: "default"},
			})

			// Retrying cannot help, so the reconciliation reports no error and
			// waits for the Secret watch or the resync timer.
			require.NoError(t, err)
			assert.Equal(t, defaultRequeueAfter, result.RequeueAfter)
			assert.Empty(t, provider.created, "a monitor must not be written without its header")
			assert.Empty(t, provider.updated)

			synced := &mackerelv1alpha1.ExternalMonitor{}
			require.NoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), synced))
			ready := meta.FindStatusCondition(synced.Status.Conditions, operatorstatus.ConditionReady)
			require.NotNil(t, ready)
			assert.Equal(t, metav1.ConditionFalse, ready.Status)
			assert.Equal(t, operatorstatus.ReasonSecretNotFound, ready.Reason)
		})
	}
}

func TestExternalMonitorReconciler_ReconcileUpdatesRotatedSecretValue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newExternalMonitorTestScheme(t)
	cr := secretBackedExternalMonitor()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "api-credentials", Namespace: "default"},
		Data:       map[string][]byte{"token": []byte("Bearer rotated")},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr, secret).
		WithStatusSubresource(&mackerelv1alpha1.ExternalMonitor{}).
		Build()

	// The monitor in Mackerel still carries the value from before the rotation,
	// marked with the hash that value produced.
	previous, err := (&ExternalMonitorReconciler{OwnerID: "prod", HashLength: 7}).
		externalMonitorSource(map[string]string{"Authorization": "Bearer token"}).
		FromExternalMonitor(cr)
	require.NoError(t, err)
	provider := newFakeExternalMonitorProvider()
	provider.monitors["mon-1"] = actualFromDesired("mon-1", previous, ownership.ApplyMarker("", ownership.Marker{
		Resource: previous.Resource,
		Owner:    previous.Owner,
		Hash:     previous.Hash,
	}), mackerelDefaultHeaders())

	reconciler := &ExternalMonitorReconciler{
		Client:     k8sClient,
		Scheme:     scheme,
		Provider:   provider,
		OwnerID:    "prod",
		Policy:     "sync",
		HashLength: 7,
	}

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "api-health", Namespace: "default"},
	})

	require.NoError(t, err)
	require.Len(t, provider.updated, 1)
	require.NotNil(t, provider.updated[0].Headers)
	assert.Equal(t, []monitor.HeaderField{{Name: "Authorization", Value: "Bearer rotated"}}, *provider.updated[0].Headers)
}

func TestExternalMonitorReconciler_ReconcileDeletesMonitorWhenSecretIsAlreadyGone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newExternalMonitorTestScheme(t)
	cr := secretBackedExternalMonitor(externalMonitorFinalizer)
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(&mackerelv1alpha1.ExternalMonitor{}).
		Build()
	provider := newFakeExternalMonitorProvider()
	provider.monitors["mon-1"] = monitor.ActualExternalMonitor{
		ID:     "mon-1",
		Name:   "default/api-health",
		URL:    "https://example.com/healthz",
		Method: "GET",
		Memo: ownership.BuildMarker(ownership.Marker{
			Resource: "externalmonitor/default/api-health",
			Owner:    "prod",
			Hash:     "oldhash",
		}),
	}
	reconciler := &ExternalMonitorReconciler{
		Client:     k8sClient,
		Scheme:     scheme,
		Provider:   provider,
		OwnerID:    "prod",
		Policy:     "sync",
		HashLength: 7,
	}
	require.NoError(t, k8sClient.Delete(ctx, cr))

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "api-health", Namespace: "default"},
	})

	require.NoError(t, err)
	assert.True(t, provider.deleted["mon-1"], "a Secret deleted before its ExternalMonitor must not orphan the monitor")
}

func TestExternalMonitorReconciler_ExternalMonitorsForSecret(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newExternalMonitorTestScheme(t)
	referencing := secretBackedExternalMonitor()
	otherNamespace := secretBackedExternalMonitor()
	otherNamespace.Namespace = "other"
	inline := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "inline", Namespace: "default"},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL:     "https://example.com/healthz",
			Headers: new([]mackerelv1alpha1.HeaderField{{Name: "X-Request-Source", Value: new("mackerel-operator")}}),
		},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(referencing, otherNamespace, inline).
		WithIndex(&mackerelv1alpha1.ExternalMonitor{}, secretRefIndexKey, func(obj client.Object) []string {
			return secretNamesForExternalMonitor(obj.(*mackerelv1alpha1.ExternalMonitor))
		}).
		Build()
	reconciler := &ExternalMonitorReconciler{Client: k8sClient, Scheme: scheme}

	requests := reconciler.externalMonitorsForSecret(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "api-credentials", Namespace: "default"},
	})

	assert.Equal(t, []reconcile.Request{{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "api-health"},
	}}, requests)
}
