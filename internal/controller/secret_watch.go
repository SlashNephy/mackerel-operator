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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretCacheOptions limits only Secret informers, leaving ExternalMonitor discovery unchanged.
func SecretCacheOptions(namespaces string, disabled bool) (cache.Options, error) {
	if disabled && namespaces != "" {
		return cache.Options{}, fmt.Errorf("secret-watch-namespaces and disable-secret-watch are mutually exclusive")
	}
	if namespaces == "" {
		return cache.Options{}, nil
	}
	allowed := map[string]cache.Config{}
	for value := range strings.SplitSeq(namespaces, ",") {
		ns := strings.TrimSpace(value)
		if errors := validation.IsDNS1123Label(ns); len(errors) != 0 {
			return cache.Options{}, fmt.Errorf("invalid Secret watch namespace %q: %s", ns, strings.Join(errors, "; "))
		}
		allowed[ns] = cache.Config{}
	}
	return cache.Options{ByObject: map[client.Object]cache.ByObject{&corev1.Secret{}: {Namespaces: allowed}}}, nil
}
