/*
Copyright 2021.

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

package console

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	consolev1 "github.com/openshift/api/console/v1"
)

const testNamespace = "test-namespace"

var _ = Describe("Console plugin", func() {
	var (
		cl  client.Client
		ctx = context.Background()
		log = ctrl.Log.WithName("console-plugin-unit-test")
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(consolev1.Install(scheme)).To(Succeed())
		cl = fake.NewClientBuilder().WithScheme(scheme).Build()
	})

	verifyPlugin := func() {
		cp := &consolev1.ConsolePlugin{}
		ExpectWithOffset(1, cl.Get(ctx, client.ObjectKey{Name: PluginName}, cp)).To(Succeed())
		ExpectWithOffset(1, cp.Spec.Backend.Type).To(Equal(consolev1.Service))
		ExpectWithOffset(1, cp.Spec.Backend.Service).To(Equal(&consolev1.ConsolePluginService{
			Name:      ServiceName,
			Namespace: testNamespace,
			Port:      ServicePort,
			BasePath:  "/",
		}))
	}

	When("the console plugin doesn't exist", func() {
		It("should create it", func() {
			Expect(CreateOrUpdatePlugin(ctx, cl, testNamespace, log)).To(Succeed())
			verifyPlugin()
		})
	})

	When("the console plugin exists with an outdated spec", func() {
		BeforeEach(func() {
			outdated := &consolev1.ConsolePlugin{
				ObjectMeta: metav1.ObjectMeta{Name: PluginName},
				Spec: consolev1.ConsolePluginSpec{
					DisplayName: "outdated",
					Backend: consolev1.ConsolePluginBackend{
						Type: consolev1.Service,
						Service: &consolev1.ConsolePluginService{
							Name:      "outdated",
							Namespace: "outdated",
							Port:      1234,
							BasePath:  "/",
						},
					},
				},
			}
			Expect(cl.Create(ctx, outdated)).To(Succeed())
		})
		It("should update it", func() {
			Expect(CreateOrUpdatePlugin(ctx, cl, testNamespace, log)).To(Succeed())
			verifyPlugin()
		})
	})
})
