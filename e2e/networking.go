package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = ginkgo.Describe("AI Inference", func() {
	f := framework.NewDefaultFramework("ai-inference")
	f.SkipNamespaceCreation = true

	/*
		Release: v1.34
		Testname: Kubernetes Gateway API Support
		Description: Support the Kubernetes Gateway API with an implementation for advanced traffic management
		for inference services, which enables capabilities like weighted traffic splitting, header-based routing
		(for OpenAI protocol headers), and optional integration with service meshes.
	*/
	framework.ConformanceIt("gateway crds should be available", func(ctx context.Context) {
		apiExtensionClient, err := apiextclientset.NewForConfig(f.ClientConfig())
		framework.ExpectNoError(err)

		crds, err := apiExtensionClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
		framework.ExpectNoError(err)

		expectedCrds := sets.New(
			"gatewayclasses.gateway.networking.k8s.io",
			"gateways.gateway.networking.k8s.io",
			"httproutes.gateway.networking.k8s.io",
			"grpcroutes.gateway.networking.k8s.io",
			"referencegrants.gateway.networking.k8s.io")

		foundCrds := sets.New[string]()
		for _, crd := range crds.Items {
			if !expectedCrds.Has(crd.Name) {
				continue
			}
			foundCrds.Insert(crd.Name)
			// Check if the CRD has accepted and established conditions which means Gateway APIs is ready to use
			gomega.Expect(crd.Status.Conditions).To(gstruct.MatchElements(
				func(condition any) string {
					return string(condition.(apiextv1.CustomResourceDefinitionCondition).Type)
				},
				gstruct.IgnoreExtras,
				gstruct.Elements{
					"NamesAccepted": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Status": gomega.Equal(apiextv1.ConditionTrue),
					}),
					"Established": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Status": gomega.Equal(apiextv1.ConditionTrue),
					}),
				},
			))
		}
		gomega.Expect(foundCrds).To(gomega.Equal(expectedCrds), "missing gateway crds: %v", sets.List(expectedCrds.Difference(foundCrds)))
	})
})
