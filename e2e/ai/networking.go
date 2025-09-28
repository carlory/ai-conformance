package ai

import (
	"context"

	"github.com/onsi/gomega"
	apiextclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
	e2ecrd "github.com/carlory/ai-conformance/e2e/util/framework/crd"
)

var _ = WGDescribe("AI Inference", func() {
	f := framework.NewDefaultFramework("ai-inference")
	f.SkipNamespaceCreation = true

	/*
		Release: v1.34
		Testname: Kubernetes Gateway API Support
		Description: Kubernetes Gateway API MUST be installed, including gatewayclasses, gateways, httproutes, grpcroutes,
		and referencegrants in the gateways.networking.k8s.io group. And these CRDs MUST have NamesAccepted and Established
		conditions with True status.
	*/
	frameworkutil.AIConformanceIt("gateway crds should be available", func(ctx context.Context) {
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
			err = e2ecrd.WaitForCrdEstablishedAndNamesAccepted(ctx, apiExtensionClient, crd.GetName())
			framework.ExpectNoError(err, "error when waiting for CRD %s to be established and names accepted", crd.GetName())
		}
		gomega.Expect(foundCrds).To(gomega.Equal(expectedCrds), "missing gateway crds: %v", sets.List(expectedCrds.Difference(foundCrds)))
	})
})
