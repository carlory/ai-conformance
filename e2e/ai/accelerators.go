package ai

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
)

var _ = WGDescribe("DRA Support", func() {
	f := framework.NewDefaultFramework("dra-support")
	f.SkipNamespaceCreation = true

	ginkgo.BeforeEach(func(ctx context.Context) {
		e2eskipper.SkipUnlessServerVersionGTE(utilversion.MustParseSemantic("v1.34.0"), f.ClientSet.Discovery())
	})

	/*
		Release: v1.34
		Testname: Dynamic Resource Allocation (DRA) API Available
		Description: The resources.k8s.io/v1 API group MUST be served by the API server.
	*/
	frameworkutil.AIConformanceIt("should support DRA", func(ctx context.Context) {
		resources, err := f.ClientSet.Discovery().ServerResourcesForGroupVersion("resource.k8s.io/v1")
		framework.ExpectNoError(err)
		gomega.Expect(resources).NotTo(gomega.BeNil())
		gomega.Expect(resources.APIResources).NotTo(gomega.BeEmpty())
	})
})
