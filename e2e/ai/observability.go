package ai

import (
	"context"
	"fmt"

	"github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
)

var _ = WGDescribe("Accelerator Metrics", func() {
	f := framework.NewDefaultFramework("accelerator-metrics")
	f.SkipNamespaceCreation = true

	/*
		Release: v1.34
		Testname: Accelerator Metrics
		Description: For supported accelerator types, the platform must allow for the installation and successful operation
		of at least one accelerator metrics solution that exposes fine-grained performance metrics via a standardized,
		machine-readable metrics endpoint. This must include a core set of metrics for per-accelerator utilization and memory usage.
		Additionally, other relevant metrics such as temperature, power draw, and interconnect bandwidth should be exposed if the
		underlying hardware or virtualization layer makes them available. The list of metrics should align with emerging standards,
		such as OpenTelemetry metrics, to ensure interoperability. The platform may provide a managed solution, but this is not required for conformance.
	*/
	frameworkutil.AIConformanceIt("should support accelerator metrics", func(ctx context.Context) {
		jobName := "nvidia-dcgm-exporter"

		queryString := fmt.Sprintf(`count by (__name__) ({job="%s"})`, jobName)

		raw, err := QueryPrometheus(PrometheusQueryParams{
			RestClient:            f.ClientSet.CoreV1().RESTClient(),
			PrometheusmNamespace:  "monitoring",
			PrometheusServiceName: "kube-prometheus-stack-prometheus",
			Query:                 queryString,
		})
		framework.ExpectNoError(err, "error when getting prometheus query")
		gomega.Expect(raw).NotTo(gomega.BeEmpty(), "expected prometheus query result to be not empty")
	})
})
