package ai

import (
	"context"

	"github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
)

var _ = WGDescribe("DRA Support", func() {
	f := framework.NewDefaultFramework("dra-support")
	f.SkipNamespaceCreation = true

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
