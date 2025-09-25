package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = ginkgo.Describe("DRA Support", func() {
	f := framework.NewDefaultFramework("dra-support")
	f.SkipNamespaceCreation = true

	/*
		Release: v1.34
		Testname: Dynamic Resource Allocation (DRA) Support
		Description: Support Dynamic Resource Allocation (DRA) APIs to enable more flexible and fine-grained resource requests beyond simple counts.
	*/
	framework.ConformanceIt("should support DRA", func(ctx context.Context) {
		resources, err := f.ClientSet.Discovery().ServerResourcesForGroupVersion("resource.k8s.io/v1")
		framework.ExpectNoError(err)
		gomega.Expect(resources).NotTo(gomega.BeNil())
		gomega.Expect(resources.APIResources).NotTo(gomega.BeEmpty())
	})
})
