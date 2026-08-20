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
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
)

// secretRefIndexKey indexes ExternalMonitors by the names of the Secrets their
// headers reference, so that a Secret event can be mapped back to the
// ExternalMonitors that have to be reconciled.
const secretRefIndexKey = ".spec.headers.valueFrom.secretKeyRef.name"

// errSecretKeyMissing reports a Secret that exists but does not carry the
// referenced key. It is treated like a missing Secret: the reconciliation is
// blocked rather than retried, because only a user edit can resolve it.
var errSecretKeyMissing = errors.New("secret key not found")

// resolveHeaderValues reads the values of every header declared with valueFrom.
// The returned map is keyed by header name and is always non-nil, which is what
// tells the source layer that this caller resolves headers.
//
// Secrets are read through APIReader rather than the manager cache: caching
// them would keep the plain text of every Secret in the cluster in the
// operator's memory, while the reconciliation only needs the few keys a CR
// actually references.
func (r *ExternalMonitorReconciler) resolveHeaderValues(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor) (map[string]string, error) {
	if cr.Spec.Headers == nil {
		return map[string]string{}, nil
	}

	headers := *cr.Spec.Headers
	values := make(map[string]string, len(headers))
	secrets := make(map[string]*corev1.Secret)

	for i := range headers {
		header := &headers[i]
		if header.ValueFrom == nil {
			continue
		}

		ref := header.ValueFrom.SecretKeyRef
		if ref == nil {
			return nil, fmt.Errorf("header %q has valueFrom without secretKeyRef", header.Name)
		}

		secret, ok := secrets[ref.Name]
		if !ok {
			secret = &corev1.Secret{}
			key := types.NamespacedName{Namespace: cr.Namespace, Name: ref.Name}
			if err := r.apiReader().Get(ctx, key, secret); err != nil {
				return nil, fmt.Errorf("get secret %s for header %q: %w", key, header.Name, err)
			}
			secrets[ref.Name] = secret
		}

		value, ok := secret.Data[ref.Key]
		if !ok {
			return nil, fmt.Errorf("secret %s/%s for header %q: %w: %s", cr.Namespace, ref.Name, header.Name, errSecretKeyMissing, ref.Key)
		}
		values[header.Name] = string(value)
	}

	return values, nil
}

// isUnresolvableSecretRef reports whether err leaves the CR waiting for a user
// edit. Such an error must not be returned from Reconcile: retrying with
// backoff would never succeed, and the Secret watch already requeues the CR
// once the reference becomes resolvable.
func isUnresolvableSecretRef(err error) bool {
	return apierrors.IsNotFound(err) || errors.Is(err, errSecretKeyMissing)
}

// secretNamesForExternalMonitor lists the Secrets an ExternalMonitor reads its
// header values from. It backs both the field index and the deduplication of
// the Secret reads.
func secretNamesForExternalMonitor(cr *mackerelv1alpha1.ExternalMonitor) []string {
	if cr.Spec.Headers == nil {
		return nil
	}

	headers := *cr.Spec.Headers
	names := make([]string, 0, len(headers))
	for i := range headers {
		header := &headers[i]
		if header.ValueFrom == nil || header.ValueFrom.SecretKeyRef == nil {
			continue
		}
		if name := header.ValueFrom.SecretKeyRef.Name; name != "" && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}
