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
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type secretWatchTransport struct {
	http.RoundTripper
	requests *atomic.Int64
}

func (r secretWatchTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/api/v1/secrets" || strings.HasSuffix(req.URL.Path, "/secrets") {
		r.requests.Add(1)
	}
	return r.RoundTripper.RoundTrip(req)
}

var _ = Describe("Secret watch configuration", func() {
	for _, disabled := range []bool{false, true} {
		It("starts and syncs the controller with Secret watches disabled="+strconv.FormatBool(disabled), func() {
			var requests atomic.Int64
			restConfig := rest.CopyConfig(cfg)
			restConfig.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
				return secretWatchTransport{RoundTripper: rt, requests: &requests}
			}
			mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
				Scheme:                 scheme.Scheme,
				Controller:             config.Controller{SkipNameValidation: new(true)},
				Metrics:                metricsserver.Options{BindAddress: "0"},
				HealthProbeBindAddress: "0",
			})
			Expect(err).NotTo(HaveOccurred())
			runCtx, stop := context.WithTimeout(ctx, 20*time.Second)
			defer stop()
			r := &ExternalMonitorReconciler{
				Client: mgr.GetClient(), APIReader: mgr.GetAPIReader(), Scheme: mgr.GetScheme(),
				Provider: newFakeExternalMonitorProvider(), OwnerID: "test", Policy: "sync", HashLength: 7,
				DisableSecretWatch: disabled,
			}
			Expect(r.SetupWithManager(runCtx, mgr)).To(Succeed())
			done := make(chan error, 1)
			go func() { done <- mgr.Start(runCtx) }()
			defer func() {
				stop()
				Eventually(done, 10*time.Second).Should(Receive(Succeed()))
			}()
			Eventually(func() bool { return mgr.GetCache().WaitForCacheSync(runCtx) }, 10*time.Second).Should(BeTrue())
			cr := &mackerelv1alpha1.ExternalMonitor{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "secret-watch-", Namespace: "default"},
				Spec:       mackerelv1alpha1.ExternalMonitorSpec{URL: "https://example.com/healthz"},
			}
			Expect(k8sClient.Create(runCtx, cr)).To(Succeed())
			defer func() {
				Expect(k8sClient.Delete(runCtx, cr)).To(Succeed())
				Eventually(func() bool {
					return apierrors.IsNotFound(k8sClient.Get(runCtx, client.ObjectKeyFromObject(cr), &mackerelv1alpha1.ExternalMonitor{}))
				}, 10*time.Second).Should(BeTrue())
			}()
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(runCtx, client.ObjectKeyFromObject(cr), cr)).To(Succeed())
				g.Expect(cr.Status.MonitorID).NotTo(BeEmpty())
			}, 10*time.Second).Should(Succeed())
			if disabled {
				Consistently(requests.Load, time.Second, 20*time.Millisecond).Should(BeZero())
			} else {
				Eventually(requests.Load, 10*time.Second).Should(BeNumerically(">", 0))
			}
		})
	}
})
