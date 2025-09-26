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
		Testname: Dynamic Resource Allocation (DRA) Support
		Description: Support Dynamic Resource Allocation (DRA) APIs to enable more flexible and fine-grained resource requests beyond simple counts.
	*/
	frameworkutil.AIConformanceIt("should support DRA", func(ctx context.Context) {
		resources, err := f.ClientSet.Discovery().ServerResourcesForGroupVersion("resource.k8s.io/v1")
		framework.ExpectNoError(err)
		gomega.Expect(resources).NotTo(gomega.BeNil())
		gomega.Expect(resources.APIResources).NotTo(gomega.BeEmpty())
	})
})
