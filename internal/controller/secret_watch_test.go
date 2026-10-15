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
	"reflect"
	"testing"
)

func TestSecretCacheOptions(t *testing.T) {
	for _, tt := range []struct {
		name, namespaces  string
		disabled, invalid bool
		want              []string
	}{
		{name: "legacy all namespaces"},
		{name: "disabled", disabled: true},
		{name: "scoped", namespaces: "argocd, authentik,argocd", want: []string{"argocd", "authentik"}},
		{name: "conflicting", namespaces: "argocd", disabled: true, invalid: true},
		{name: "empty entry", namespaces: "argocd,", invalid: true},
		{name: "invalid namespace", namespaces: "UPPER", invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := SecretCacheOptions(tt.namespaces, tt.disabled)
			if (err != nil) != tt.invalid {
				t.Fatalf("error=%v, invalid=%v", err, tt.invalid)
			}
			if err != nil {
				return
			}
			if opts.DefaultNamespaces != nil {
				t.Fatal("ExternalMonitor discovery must remain cluster-wide")
			}
			if len(tt.want) == 0 {
				if len(opts.ByObject) != 0 {
					t.Fatal("unexpected Secret cache override")
				}
				return
			}
			if len(opts.ByObject) != 1 {
				t.Fatalf("cache overrides=%d", len(opts.ByObject))
			}
			for _, obj := range opts.ByObject {
				want := map[string]bool{}
				for _, ns := range tt.want {
					want[ns] = true
				}
				got := map[string]bool{}
				for ns := range obj.Namespaces {
					got[ns] = true
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("namespaces=%v, want=%v", got, want)
				}
			}
		})
	}
}
