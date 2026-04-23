package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/SlashNephy/mackerel-operator/internal/ownership"
	mackerelprovider "github.com/SlashNephy/mackerel-operator/internal/provider"
	operatorstatus "github.com/SlashNephy/mackerel-operator/internal/status"
)

func TestExternalMonitorReconciler_ReconcileCreatesMonitor(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newExternalMonitorTestScheme(t)
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-health",
			Namespace: "default",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL:  "https://example.com/healthz",
			Memo: "human memo",
		},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
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

	require.NoError(t, err)
	assert.Equal(t, defaultRequeueAfter, result.RequeueAfter)
	require.Len(t, provider.created, 1)
	assert.Equal(t, "default/api-health", provider.created[0].Name)
	assert.Equal(t, "https://example.com/healthz", provider.created[0].URL)
	assert.Equal(t, "GET", provider.created[0].Method)
	assert.Contains(t, provider.createdMemo[0], "human memo")
	marker, ok := ownership.ParseMarker(provider.createdMemo[0])
	require.True(t, ok)
	assert.Equal(t, "externalmonitor/default/api-health", marker.Resource)
	assert.Equal(t, "prod", marker.Owner)
	assert.Len(t, marker.Hash, 7)

	synced := &mackerelv1alpha1.ExternalMonitor{}
	require.NoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), synced))
	assert.Contains(t, synced.Finalizers, externalMonitorFinalizer)
	assert.Equal(t, "mon-1", synced.Status.MonitorID)
	assert.Equal(t, "https://example.com/healthz", synced.Status.URL)
	assert.Equal(t, "default/api-health", synced.Status.MackerelMonitorName)
	assert.Equal(t, marker.Hash, synced.Status.LastAppliedHash)
	ready := meta.FindStatusCondition(synced.Status.Conditions, operatorstatus.ConditionReady)
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionTrue, ready.Status)
	assert.Equal(t, operatorstatus.ReasonSynced, ready.Reason)
}

func TestExternalMonitorReconciler_ReconcileUpdatesOwnedMonitor(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newExternalMonitorTestScheme(t)
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-health",
			Namespace: "default",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL:  "https://example.com/new",
			Memo: "new memo",
		},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(&mackerelv1alpha1.ExternalMonitor{}).
		Build()
	provider := newFakeExternalMonitorProvider()
	provider.monitors["mon-1"] = monitor.ActualExternalMonitor{
		ID:     "mon-1",
		Name:   "default/api-health",
		URL:    "https://example.com/old",
		Method: "GET",
		Memo: "operator-side memo\n" + ownership.BuildMarker(ownership.Marker{
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

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "api-health", Namespace: "default"},
	})

	require.NoError(t, err)
	require.Len(t, provider.updated, 1)
	assert.Equal(t, "mon-1", provider.updatedID[0])
	assert.Equal(t, "https://example.com/new", provider.updated[0].URL)
	assert.Contains(t, provider.updatedMemo[0], "operator-side memo")
	marker, ok := ownership.ParseMarker(provider.updatedMemo[0])
	require.True(t, ok)
	assert.Equal(t, "externalmonitor/default/api-health", marker.Resource)
	assert.Len(t, marker.Hash, 7)
}

func TestExternalMonitorReconciler_ReconcileMarksOwnershipLost(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newExternalMonitorTestScheme(t)
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-health",
			Namespace: "default",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://example.com/healthz",
		},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(&mackerelv1alpha1.ExternalMonitor{}).
		Build()
	provider := newFakeExternalMonitorProvider()
	provider.monitors["mon-1"] = monitor.ActualExternalMonitor{
		ID:     "mon-1",
		Name:   "default/api-health",
		URL:    "https://example.com/changed-by-human",
		Method: "GET",
		Memo:   "human memo",
	}
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
	assert.Empty(t, provider.updated)
	synced := &mackerelv1alpha1.ExternalMonitor{}
	require.NoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), synced))
	ready := meta.FindStatusCondition(synced.Status.Conditions, operatorstatus.ConditionReady)
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionFalse, ready.Status)
	assert.Equal(t, operatorstatus.ReasonOwnershipLost, ready.Reason)
}

func TestExternalMonitorReconciler_ReconcileRestoresMarker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newExternalMonitorTestScheme(t)
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-health",
			Namespace: "default",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://example.com/healthz",
		},
	}
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
		Memo:   "human memo",
	}
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
	require.Len(t, provider.updated, 1)
	assert.Equal(t, "mon-1", provider.updatedID[0])
	assert.Contains(t, provider.updatedMemo[0], "human memo")
	marker, ok := ownership.ParseMarker(provider.updatedMemo[0])
	require.True(t, ok)
	assert.Equal(t, "externalmonitor/default/api-health", marker.Resource)
	assert.Equal(t, "prod", marker.Owner)
}

func TestExternalMonitorReconciler_ReconcileNoopsOwnedMonitor(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newExternalMonitorTestScheme(t)
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-health",
			Namespace: "default",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://example.com/healthz",
		},
	}
	desired, desiredErr := (&ExternalMonitorReconciler{OwnerID: "prod", HashLength: 7}).externalMonitorSource().FromExternalMonitor(cr)
	require.NoError(t, desiredErr)

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(&mackerelv1alpha1.ExternalMonitor{}).
		Build()
	provider := newFakeExternalMonitorProvider()
	provider.monitors["mon-1"] = actualFromDesired("mon-1", desired, ownership.ApplyMarker("human memo", ownership.Marker{
		Resource: desired.Resource,
		Owner:    desired.Owner,
		Hash:     desired.Hash,
	}))
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
	assert.Empty(t, provider.created)
	assert.Empty(t, provider.updated)
	synced := &mackerelv1alpha1.ExternalMonitor{}
	require.NoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), synced))
	assert.Equal(t, "mon-1", synced.Status.MonitorID)
}

func TestExternalMonitorReconciler_ReconcilePrefersOwnedMarkerOverNameOrder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newExternalMonitorTestScheme(t)
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-health",
			Namespace: "default",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://example.com/healthz",
		},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(&mackerelv1alpha1.ExternalMonitor{}).
		Build()
	provider := newFakeExternalMonitorProvider()
	provider.monitorOrder = []string{"unowned", "owned"}
	provider.monitors["unowned"] = monitor.ActualExternalMonitor{
		ID:     "unowned",
		Name:   "default/api-health",
		URL:    "https://example.com/changed-by-human",
		Method: "GET",
		Memo:   "human memo",
	}
	provider.monitors["owned"] = monitor.ActualExternalMonitor{
		ID:     "owned",
		Name:   "default/api-health",
		URL:    "https://example.com/old",
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

	_, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "api-health", Namespace: "default"},
	})

	require.NoError(t, err)
	require.Len(t, provider.updatedID, 1)
	assert.Equal(t, "owned", provider.updatedID[0])
}

func TestExternalMonitorReconciler_ReconcileRateLimited(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newExternalMonitorTestScheme(t)
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-health",
			Namespace: "default",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://example.com/healthz",
		},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(&mackerelv1alpha1.ExternalMonitor{}).
		Build()
	provider := newFakeExternalMonitorProvider()
	provider.createErr = mackerelprovider.ErrRateLimited
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

	require.NoError(t, err)
	assert.Equal(t, defaultRequeueAfter, result.RequeueAfter)
	synced := &mackerelv1alpha1.ExternalMonitor{}
	require.NoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), synced))
	ready := meta.FindStatusCondition(synced.Status.Conditions, operatorstatus.ConditionReady)
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionFalse, ready.Status)
	assert.Equal(t, "ProviderError", ready.Reason)
}

func TestExternalMonitorReconciler_ReconcileDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		policy      string
		wantDeleted bool
	}{
		{
			name:        "sync policy deletes owned monitor",
			policy:      "sync",
			wantDeleted: true,
		},
		{
			name:        "upsert only policy preserves owned monitor",
			policy:      "upsert-only",
			wantDeleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			scheme := newExternalMonitorTestScheme(t)
			cr := &mackerelv1alpha1.ExternalMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "api-health",
					Namespace:  "default",
					Finalizers: []string{externalMonitorFinalizer},
				},
				Spec: mackerelv1alpha1.ExternalMonitorSpec{
					URL: "https://example.com/healthz",
				},
			}
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
				Policy:     tt.policy,
				HashLength: 7,
			}
			require.NoError(t, k8sClient.Delete(ctx, cr))

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "api-health", Namespace: "default"},
			})

			require.NoError(t, err)
			assert.Equal(t, tt.wantDeleted, provider.deleted["mon-1"])
			synced := &mackerelv1alpha1.ExternalMonitor{}
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), synced)
			if apierrors.IsNotFound(err) {
				return
			}
			require.NoError(t, err)
			assert.NotContains(t, synced.Finalizers, externalMonitorFinalizer)
		})
	}
}

func newExternalMonitorTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, mackerelv1alpha1.AddToScheme(scheme))
	return scheme
}

type fakeExternalMonitorProvider struct {
	monitors     map[string]monitor.ActualExternalMonitor
	monitorOrder []string
	created      []monitor.DesiredExternalMonitor
	createdMemo  []string
	createErr    error
	updatedID    []string
	updated      []monitor.DesiredExternalMonitor
	updatedMemo  []string
	updateErr    error
	deleted      map[string]bool
}

func newFakeExternalMonitorProvider() *fakeExternalMonitorProvider {
	return &fakeExternalMonitorProvider{
		monitors: make(map[string]monitor.ActualExternalMonitor),
		deleted:  make(map[string]bool),
	}
}

func (p *fakeExternalMonitorProvider) GetExternalMonitor(_ context.Context, id string) (*monitor.ActualExternalMonitor, error) {
	actual, ok := p.monitors[id]
	if !ok {
		return nil, mackerelprovider.ErrNotFound
	}
	return &actual, nil
}

func (p *fakeExternalMonitorProvider) ListExternalMonitors(_ context.Context) ([]monitor.ActualExternalMonitor, error) {
	actuals := make([]monitor.ActualExternalMonitor, 0, len(p.monitors))
	for _, id := range p.monitorOrder {
		actual, ok := p.monitors[id]
		if ok {
			actuals = append(actuals, actual)
		}
	}
	if len(p.monitorOrder) > 0 {
		return actuals, nil
	}
	for _, actual := range p.monitors {
		actuals = append(actuals, actual)
	}
	return actuals, nil
}

func (p *fakeExternalMonitorProvider) CreateExternalMonitor(_ context.Context, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error) {
	if p.createErr != nil {
		return nil, p.createErr
	}

	p.created = append(p.created, desired)
	p.createdMemo = append(p.createdMemo, memo)

	actual := actualFromDesired("mon-1", desired, memo)
	p.monitors[actual.ID] = actual
	return &actual, nil
}

func (p *fakeExternalMonitorProvider) UpdateExternalMonitor(_ context.Context, id string, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error) {
	if p.updateErr != nil {
		return nil, p.updateErr
	}

	p.updatedID = append(p.updatedID, id)
	p.updated = append(p.updated, desired)
	p.updatedMemo = append(p.updatedMemo, memo)

	actual := actualFromDesired(id, desired, memo)
	p.monitors[id] = actual
	return &actual, nil
}

func (p *fakeExternalMonitorProvider) DeleteExternalMonitor(_ context.Context, id string) error {
	p.deleted[id] = true
	delete(p.monitors, id)
	return nil
}

func actualFromDesired(id string, desired monitor.DesiredExternalMonitor, memo string) monitor.ActualExternalMonitor {
	return monitor.ActualExternalMonitor{
		ID:                              id,
		Name:                            desired.Name,
		Service:                         desired.Service,
		URL:                             desired.URL,
		Method:                          desired.Method,
		NotificationInterval:            desired.NotificationInterval,
		ExpectedStatusCode:              desired.ExpectedStatusCode,
		ContainsString:                  desired.ContainsString,
		ResponseTimeWarning:             desired.ResponseTimeWarning,
		ResponseTimeCritical:            desired.ResponseTimeCritical,
		CertificationExpirationWarning:  desired.CertificationExpirationWarning,
		CertificationExpirationCritical: desired.CertificationExpirationCritical,
		Memo:                            memo,
	}
}
