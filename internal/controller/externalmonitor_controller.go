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

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/SlashNephy/mackerel-operator/internal/ownership"
	"github.com/SlashNephy/mackerel-operator/internal/planner"
	"github.com/SlashNephy/mackerel-operator/internal/provider"
	"github.com/SlashNephy/mackerel-operator/internal/source"
	operatorstatus "github.com/SlashNephy/mackerel-operator/internal/status"
)

const (
	externalMonitorFinalizer = "externalmonitor.mackerel.starry.blue/finalizer"
	defaultRequeueAfter      = time.Minute
	defaultHashLength        = 7
	defaultOwnerID           = "default"
	defaultPolicy            = "upsert-only"
)

// ExternalMonitorReconciler reconciles a ExternalMonitor object
type ExternalMonitorReconciler struct {
	client.Client
	// APIReader reads past the informer cache. It refreshes a resource after a
	// conflict, where a cached read may still return the very version that caused
	// it, and it reads the Secrets referenced by header values, which are
	// deliberately not cached. Falls back to Client when nil.
	APIReader  client.Reader
	Scheme     *runtime.Scheme
	Provider   provider.ExternalMonitorProvider
	OwnerID    string
	Policy     string
	HashLength int
}

var errProviderNil = errors.New("external monitor provider is nil")
var errAmbiguousExternalMonitor = errors.New("ambiguous external monitor candidates")

// +kubebuilder:rbac:groups=mackerel.starry.blue,resources=externalmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mackerel.starry.blue,resources=externalmonitors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mackerel.starry.blue,resources=externalmonitors/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *ExternalMonitorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if r.Provider == nil {
		return ctrl.Result{}, errProviderNil
	}

	cr := &mackerelv1alpha1.ExternalMonitor{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !cr.DeletionTimestamp.IsZero() {
		desired, err := r.externalMonitorDeletionSource().FromExternalMonitor(cr)
		if err != nil {
			return ctrl.Result{}, r.removeFinalizer(ctx, cr)
		}
		actual, err := r.findActual(ctx, cr.Status.MonitorID, desired)
		if err != nil {
			return ctrl.Result{}, err
		}
		return r.reconcileDelete(ctx, cr, desired, actual)
	}

	headerValues, err := r.resolveHeaderValues(ctx, cr)
	if err != nil {
		// A missing Secret or key is a user-visible configuration problem, not a
		// transient failure, so the reconciliation stops here instead of writing
		// a monitor without the header. The Secret watch requeues the CR as soon
		// as the reference becomes resolvable.
		if isUnresolvableSecretRef(err) {
			log.V(1).Info("blocked on an unresolvable header value", "error", err.Error())
			return ctrl.Result{RequeueAfter: defaultRequeueAfter}, r.patchStatus(ctx, cr, func() {
				operatorstatus.MarkError(cr, operatorstatus.ReasonSecretNotFound, err.Error())
			})
		}
		if statusErr := r.patchStatus(ctx, cr, func() {
			operatorstatus.MarkError(cr, operatorstatus.ReasonSecretError, err.Error())
		}); statusErr != nil {
			return ctrl.Result{}, errors.Join(err, statusErr)
		}
		return ctrl.Result{}, err
	}

	desired, err := r.externalMonitorSource(headerValues).FromExternalMonitor(cr)
	if err != nil {
		return ctrl.Result{}, r.patchStatus(ctx, cr, func() {
			operatorstatus.MarkInvalidSpec(cr, err.Error())
		})
	}

	if !controllerutil.ContainsFinalizer(cr, externalMonitorFinalizer) {
		if err := r.addFinalizer(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
	}

	actual, err := r.findActual(ctx, cr.Status.MonitorID, desired)
	if err != nil {
		if statusErr := r.patchStatus(ctx, cr, func() {
			operatorstatus.MarkError(cr, "ProviderError", err.Error())
		}); statusErr != nil {
			return ctrl.Result{}, errors.Join(err, statusErr)
		}
		return ctrl.Result{}, err
	}

	decision := planner.Plan(planner.PlanInput{
		Desired: desired,
		Actual:  actual,
	})
	log.V(1).Info("planned external monitor reconciliation", "action", decision.Action, "reason", decision.Reason)

	marker := ownership.Marker{
		Resource: desired.Resource,
		Owner:    desired.Owner,
		Hash:     desired.Hash,
	}

	var synced *monitor.ActualExternalMonitor
	var applied bool
	switch decision.Action {
	case planner.ActionCreate:
		memo := ownership.ApplyMarker(desired.Memo, marker)
		synced, err = r.Provider.CreateExternalMonitor(ctx, desired, memo)
		applied = true
	case planner.ActionUpdate:
		if actual == nil {
			return ctrl.Result{}, fmt.Errorf("planner selected update without actual monitor")
		}
		memo := ownership.ApplyMarker(actual.Memo, marker)
		synced, err = r.Provider.UpdateExternalMonitor(ctx, actual.ID, desired, memo)
		applied = true
	case planner.ActionRestoreMarker:
		if actual == nil {
			return ctrl.Result{}, fmt.Errorf("planner selected marker restore without actual monitor")
		}
		memo := ownership.ApplyMarker(actual.Memo, marker)
		synced, err = r.Provider.UpdateExternalMonitor(ctx, actual.ID, desired, memo)
		applied = true
	case planner.ActionNoop:
		synced = actual
	case planner.ActionOwnershipLost:
		if err := r.patchStatus(ctx, cr, func() {
			operatorstatus.MarkOwnershipLost(cr, decision.Reason)
		}); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
	default:
		return ctrl.Result{}, fmt.Errorf("unsupported planner action: %s", decision.Action)
	}

	if err != nil {
		if statusErr := r.patchStatus(ctx, cr, func() {
			operatorstatus.MarkError(cr, "ProviderError", err.Error())
		}); statusErr != nil {
			return ctrl.Result{}, errors.Join(err, statusErr)
		}
		if errors.Is(err, provider.ErrRateLimited) {
			return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
		}
		return ctrl.Result{}, err
	}
	if synced == nil {
		return ctrl.Result{}, fmt.Errorf("provider returned nil external monitor")
	}

	// RequeueAfter is ignored whenever the error is non-nil, so the two must not be
	// returned together: controller-runtime logs a warning for that combination.
	if err := r.patchStatus(ctx, cr, func() {
		operatorstatus.MarkReady(cr, operatorstatus.SyncResult{
			MonitorID: synced.ID,
			Hash:      desired.Hash,
			URL:       synced.URL,
			Name:      synced.Name,
			Applied:   applied,
		})
	}); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
}

// patchStatus applies mark to cr and persists only the resulting status diff as a
// merge patch. Unlike Update, a merge patch carries no resourceVersion precondition,
// so a spec change landing while the provider call is in flight does not turn into a
// conflict error.
//
// Reconcile runs on a timer, so most invocations find nothing to change. Skipping the
// API call in that case keeps a steady-state resource from being rewritten once per
// resync interval.
func (r *ExternalMonitorReconciler) patchStatus(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor, mark func()) error {
	before := cr.DeepCopy()
	patch := client.MergeFrom(before)
	mark()
	if apiequality.Semantic.DeepEqual(before.Status, cr.Status) {
		return nil
	}
	return r.Status().Patch(ctx, cr, patch)
}

func (r *ExternalMonitorReconciler) addFinalizer(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor) error {
	return r.patchFinalizers(ctx, cr, func() {
		controllerutil.AddFinalizer(cr, externalMonitorFinalizer)
	})
}

func (r *ExternalMonitorReconciler) removeFinalizer(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor) error {
	// A resource that has already vanished carries no finalizer, which is the goal here.
	// The same tolerance must not be granted to addFinalizer: continuing to reconcile a
	// resource that no longer exists would leave a monitor behind in Mackerel.
	return client.IgnoreNotFound(r.patchFinalizers(ctx, cr, func() {
		controllerutil.RemoveFinalizer(cr, externalMonitorFinalizer)
	}))
}

// patchFinalizers persists a finalizer change as a minimal patch instead of sending
// the whole object. The optimistic lock is deliberate: a merge patch replaces
// metadata.finalizers wholesale, so without it a finalizer added concurrently by
// another controller would be dropped.
//
// That lock also means the patch is rejected once the resourceVersion read at the
// beginning of Reconcile has moved on. On the deletion path the provider call sits
// between that read and this write, so the window is wide enough to matter. Conflicts
// are therefore resolved here rather than deferred to the next reconciliation: the
// resource is re-read past the cache, which could otherwise hand back the very version
// that just lost, and the patch is rebuilt on top of the current one. mutate must stay
// idempotent because it is replayed on every attempt.
func (r *ExternalMonitorReconciler) patchFinalizers(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor, mutate func()) error {
	var attempted bool
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if attempted {
			if err := r.apiReader().Get(ctx, client.ObjectKeyFromObject(cr), cr); err != nil {
				return err
			}
		}
		attempted = true

		patch := client.MergeFromWithOptions(cr.DeepCopy(), client.MergeFromWithOptimisticLock{})
		mutate()
		return r.Patch(ctx, cr, patch)
	})
}

func (r *ExternalMonitorReconciler) apiReader() client.Reader {
	if r.APIReader == nil {
		return r.Client
	}
	return r.APIReader
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalMonitorReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &mackerelv1alpha1.ExternalMonitor{}, secretRefIndexKey, func(obj client.Object) []string {
		cr, ok := obj.(*mackerelv1alpha1.ExternalMonitor)
		if !ok {
			return nil
		}
		return secretNamesForExternalMonitor(cr)
	}); err != nil {
		return fmt.Errorf("index external monitors by referenced secret: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&mackerelv1alpha1.ExternalMonitor{}).
		// Only the metadata of Secrets is cached. The watch exists to notice a
		// rotation, and the values themselves are read on demand through
		// APIReader, so there is no reason to mirror every Secret payload in
		// the operator's memory.
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.externalMonitorsForSecret), builder.OnlyMetadata).
		Named("externalmonitor").
		Complete(r)
}

// externalMonitorsForSecret maps a Secret event to the ExternalMonitors that
// read a header value from it. Secret references are namespace-local, so the
// lookup is scoped to the namespace of the Secret.
func (r *ExternalMonitorReconciler) externalMonitorsForSecret(ctx context.Context, secret client.Object) []ctrl.Request {
	var list mackerelv1alpha1.ExternalMonitorList
	if err := r.List(ctx, &list,
		client.InNamespace(secret.GetNamespace()),
		client.MatchingFields{secretRefIndexKey: secret.GetName()},
	); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list external monitors referencing a secret",
			"namespace", secret.GetNamespace(), "secret", secret.GetName())
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&list.Items[i])})
	}
	return requests
}

func (r *ExternalMonitorReconciler) findActual(ctx context.Context, monitorID string, desired monitor.DesiredExternalMonitor) (*monitor.ActualExternalMonitor, error) {
	if monitorID != "" {
		actual, err := r.Provider.GetExternalMonitor(ctx, monitorID)
		if err == nil && actual != nil {
			return actual, nil
		}
		if err != nil && !errors.Is(err, provider.ErrNotFound) {
			return nil, err
		}
	}

	actuals, err := r.Provider.ListExternalMonitors(ctx)
	if err != nil {
		return nil, err
	}
	var nameMatch *monitor.ActualExternalMonitor
	for i := range actuals {
		if marker, ok := ownership.ParseMarker(actuals[i].Memo); ok &&
			marker.Owner == desired.Owner &&
			marker.Resource == desired.Resource {
			return &actuals[i], nil
		}
		if actuals[i].Name != desired.Name {
			continue
		}
		if nameMatch != nil {
			return nil, errAmbiguousExternalMonitor
		}
		nameMatch = &actuals[i]
	}

	return nameMatch, nil
}

func (r *ExternalMonitorReconciler) reconcileDelete(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor, desired monitor.DesiredExternalMonitor, actual *monitor.ActualExternalMonitor) (ctrl.Result, error) {
	decision := planner.PlanDelete(actual, desired.Owner, desired.Resource, r.policy())
	switch decision.Action {
	case planner.ActionDelete:
		if err := r.Provider.DeleteExternalMonitor(ctx, actual.ID); err != nil && !errors.Is(err, provider.ErrNotFound) {
			return ctrl.Result{}, err
		}
	case planner.ActionSkipDelete:
	default:
		return ctrl.Result{}, fmt.Errorf("unsupported delete planner action: %s", decision.Action)
	}

	return ctrl.Result{}, r.removeFinalizer(ctx, cr)
}

func (r *ExternalMonitorReconciler) externalMonitorSource(headerValues map[string]string) source.ExternalMonitorSource {
	return source.ExternalMonitorSource{
		OwnerID:      r.ownerID(),
		HashLength:   r.hashLength(),
		HeaderValues: headerValues,
	}
}

// externalMonitorDeletionSource builds the desired state used while finalizing.
// It leaves HeaderValues nil on purpose: the deletion path only needs Name,
// Owner and Resource to locate the monitor, and resolving Secrets here would
// turn a Secret deleted before its ExternalMonitor into a monitor orphaned in
// Mackerel, because a failure to build the desired state drops the finalizer.
func (r *ExternalMonitorReconciler) externalMonitorDeletionSource() source.ExternalMonitorSource {
	return source.ExternalMonitorSource{
		OwnerID:    r.ownerID(),
		HashLength: defaultHashLength,
	}
}

func (r *ExternalMonitorReconciler) ownerID() string {
	if r.OwnerID == "" {
		return defaultOwnerID
	}
	return r.OwnerID
}

func (r *ExternalMonitorReconciler) policy() string {
	if r.Policy == "" {
		return defaultPolicy
	}
	return r.Policy
}

func (r *ExternalMonitorReconciler) hashLength() int {
	if r.HashLength == 0 {
		return defaultHashLength
	}
	return r.HashLength
}
