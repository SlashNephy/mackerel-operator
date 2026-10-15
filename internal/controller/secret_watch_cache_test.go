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
	"fmt"
	"net/http"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type secretScopeTransport struct{ http.RoundTripper }

func (r secretScopeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/api/v1/secrets" || req.URL.Path == "/api/v1/namespaces/outside/secrets" {
		return nil, fmt.Errorf("out-of-scope Secret request: %s", req.URL.Path)
	}
	return r.RoundTripper.RoundTrip(req)
}

func TestScopedSecretMetadataCache(t *testing.T) {
	env := &envtest.Environment{}
	cfg, err := env.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Error(err)
		}
	})
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	admin, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, ns := range []string{"allowed", "outside"} {
		if err := admin.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); err != nil {
			t.Fatal(err)
		}
		if err := admin.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: ns}}); err != nil {
			t.Fatal(err)
		}
	}
	scoped := rest.CopyConfig(cfg)
	scoped.WrapTransport = func(rt http.RoundTripper) http.RoundTripper { return secretScopeTransport{rt} }
	opts, err := SecretCacheOptions("allowed", false)
	if err != nil {
		t.Fatal(err)
	}
	opts.Scheme = scheme
	c, err := cache.New(scoped, opts)
	if err != nil {
		t.Fatal(err)
	}
	secret := &metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}}
	if _, err := c.GetInformer(ctx, secret, cache.BlockUntilSynced(false)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- c.Start(ctx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	if !c.WaitForCacheSync(ctx) {
		t.Fatal("Secret metadata cache did not sync with only namespace-scoped requests")
	}
	list := &metav1.PartialObjectMetadataList{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "SecretList"}}
	if err := c.List(ctx, list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].Namespace != "allowed" {
		t.Fatalf("unexpected Secret metadata: %v", list.Items)
	}
}
