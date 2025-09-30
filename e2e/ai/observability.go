package ai

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	admissionapi "k8s.io/pod-security-admission/api"

	"k8s.io/kubernetes/test/e2e/framework"
	e2egpu "k8s.io/kubernetes/test/e2e/framework/gpu"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
)

var _ = WGDescribe("Accelerator Metrics", func() {
	f := framework.NewDefaultFramework("accelerator-metrics")
	f.SkipNamespaceCreation = true

	framework.Context("nvidia gpu", func() {
		gpuNodes := []corev1.Node{}
		ginkgo.BeforeEach(func(ctx context.Context) {
			nodes, err := e2enode.GetReadySchedulableNodes(ctx, f.ClientSet)
			framework.ExpectNoError(err)

			capacity := 0
			allocatable := 0
			for _, node := range nodes.Items {
				val, ok := node.Status.Capacity[e2egpu.NVIDIAGPUResourceName]
				if !ok {
					continue
				}
				capacity += int(val.Value())
				val, ok = node.Status.Allocatable[e2egpu.NVIDIAGPUResourceName]
				if !ok {
					continue
				}
				allocatable += int(val.Value())
				gpuNodes = append(gpuNodes, node)
			}

			if capacity == 0 {
				e2eskipper.Skipf("%d ready nodes do not have any Nvidia GPU(s). Skipping...", len(nodes.Items))
			}
			if allocatable == 0 {
				e2eskipper.Skipf("%d ready nodes do not have any allocatable Nvidia GPU(s). Skipping...", len(nodes.Items))
			}
		})

		/*
			Release: v1.34
			Testname: Nvidia GPU Metrics
			Description: Verify that accelerator metrics MUST be collected from the GPU node.
		*/
		frameworkutil.AIConformanceIt("metrics should be collected from the GPU node", func(ctx context.Context) {
			// TODO: implement this test
		})
	})
})

// Provide a monitoring system capable of discovering and collecting metrics from workloads that expose them in a standard format
// (e.g. Prometheus exposition format). This ensures easy integration for collecting key metrics from common AI frameworks and servers.
var _ = WGDescribe("AI Service Metrics", func() {
	f := framework.NewDefaultFramework("ai-service-metrics")
	f.NamespacePodSecurityLevel = admissionapi.LevelRestricted

	/*
		Release: v1.34
		Testname: AI Service Metrics
		Description: Verify that AI service metrics MUST be collected from the AI service.
	*/
	frameworkutil.AIConformanceIt("metrics should be collected from the AI service", func(ctx context.Context) {
		// TODO: implement this test
	})
})
