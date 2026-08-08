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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

	desired, err := r.externalMonitorSource().FromExternalMonitor(cr)
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
	switch decision.Action {
	case planner.ActionCreate:
		memo := ownership.ApplyMarker(desired.Memo, marker)
		synced, err = r.Provider.CreateExternalMonitor(ctx, desired, memo)
	case planner.ActionUpdate:
		if actual == nil {
			return ctrl.Result{}, fmt.Errorf("planner selected update without actual monitor")
		}
		memo := ownership.ApplyMarker(actual.Memo, marker)
		synced, err = r.Provider.UpdateExternalMonitor(ctx, actual.ID, desired, memo)
	case planner.ActionRestoreMarker:
		if actual == nil {
			return ctrl.Result{}, fmt.Errorf("planner selected marker restore without actual monitor")
		}
		memo := ownership.ApplyMarker(actual.Memo, marker)
		synced, err = r.Provider.UpdateExternalMonitor(ctx, actual.ID, desired, memo)
	case planner.ActionNoop:
		synced = actual
	case planner.ActionOwnershipLost:
		return ctrl.Result{RequeueAfter: defaultRequeueAfter}, r.patchStatus(ctx, cr, func() {
			operatorstatus.MarkOwnershipLost(cr, decision.Reason)
		})
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

	return ctrl.Result{RequeueAfter: defaultRequeueAfter}, r.patchStatus(ctx, cr, func() {
		operatorstatus.MarkReady(cr, operatorstatus.SyncResult{
			MonitorID: synced.ID,
			Hash:      desired.Hash,
			URL:       synced.URL,
			Name:      synced.Name,
		})
	})
}

// patchStatus applies mark to cr and persists only the resulting status diff as a
// merge patch. Unlike Update, a merge patch carries no resourceVersion precondition,
// so a spec change landing while the provider call is in flight does not turn into a
// conflict error.
func (r *ExternalMonitorReconciler) patchStatus(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor, mark func()) error {
	patch := client.MergeFrom(cr.DeepCopy())
	mark()
	return r.Status().Patch(ctx, cr, patch)
}

func (r *ExternalMonitorReconciler) addFinalizer(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor) error {
	return r.patchFinalizers(ctx, cr, func() {
		controllerutil.AddFinalizer(cr, externalMonitorFinalizer)
	})
}

func (r *ExternalMonitorReconciler) removeFinalizer(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor) error {
	return r.patchFinalizers(ctx, cr, func() {
		controllerutil.RemoveFinalizer(cr, externalMonitorFinalizer)
	})
}

// patchFinalizers persists a finalizer change as a minimal patch instead of sending
// the whole object. The optimistic lock is deliberate: a merge patch replaces
// metadata.finalizers wholesale, so without it a finalizer added concurrently by
// another controller would be dropped. Conflicts here are rare because the patch is
// issued right after the initial Get, and they are resolved by the next reconciliation.
func (r *ExternalMonitorReconciler) patchFinalizers(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor, mutate func()) error {
	patch := client.MergeFromWithOptions(cr.DeepCopy(), client.MergeFromWithOptimisticLock{})
	mutate()
	return r.Patch(ctx, cr, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalMonitorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mackerelv1alpha1.ExternalMonitor{}).
		Named("externalmonitor").
		Complete(r)
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

func (r *ExternalMonitorReconciler) externalMonitorSource() source.ExternalMonitorSource {
	return source.ExternalMonitorSource{
		OwnerID:    r.ownerID(),
		HashLength: r.hashLength(),
	}
}

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
