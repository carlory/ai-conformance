package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"

	"k8s.io/kubernetes/test/e2e/framework"
	e2egpu "k8s.io/kubernetes/test/e2e/framework/gpu"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
	e2eautoscaling "github.com/carlory/ai-conformance/e2e/util/framework/autoscaling"
	prometheusutil "github.com/carlory/ai-conformance/e2e/util/prometheus"
)

var _ = WGDescribe("Accelerator Metrics", func() {
	f := framework.NewDefaultFramework("accelerator-metrics")
	f.SkipNamespaceCreation = true
	const timeToWait = 15 * time.Minute

	framework.Context("nvidia gpu", func() {
		ginkgo.BeforeEach(func(ctx context.Context) {
			nodes, err := e2enode.GetReadyNodesIncludingTainted(ctx, f.ClientSet)
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
			Description: Query the prometheus and verify that the gpu deivce metrics MUST be collected.
		*/
		frameworkutil.AIConformanceIt("metrics should be collected from the GPU node", func(ctx context.Context) {
			ginkgo.By("Getting the Prometheus instance")
			promOpClient, err := monitoring.NewForConfig(f.ClientConfig())
			framework.ExpectNoError(err, "error when creating prometheus operator client")
			promList, err := promOpClient.MonitoringV1().Prometheuses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
			framework.ExpectNoError(err, "error when getting Prometheus list")
			gomega.Expect(promList.Items).ToNot(gomega.BeEmpty(), "at least one Prometheus should be found")
			prom := promList.Items[0]

			ginkgo.By("Query the prometheus and verify that the metrics are collected")
			metricNamePrefix := "DCGM_FI_DEV"
			query := fmt.Sprintf(`count by (__name__) ({__name__=~"^%s.*"})`, metricNamePrefix)
			err = framework.Gomega().Eventually(ctx, func(ctx context.Context) error {
				proxyRequest, err := e2eservice.GetServicesProxyRequest(f.ClientSet, f.ClientSet.CoreV1().RESTClient().Get())
				if err != nil {
					return err
				}
				req := proxyRequest.Namespace(prom.Namespace).
					Name(fmt.Sprintf("%s:http-web", prom.Name)).
					Suffix("/api/v1/query").
					Param("query", query)
				framework.Logf("Query URL: %v", *req.URL())
				data, err := req.DoRaw(ctx)
				if err != nil {
					return err
				}
				framework.Logf("Query result: %s", string(data))
				if !strings.Contains(string(data), metricNamePrefix) {
					return fmt.Errorf("metrics with prefix %q not found: %s", metricNamePrefix, string(data))
				}
				return nil
			}).WithTimeout(timeToWait).WithPolling(15 * time.Second).Should(gomega.Succeed())
			framework.ExpectNoError(err, "error when waiting for the metrics to be collected")
		})
	})
})

// Provide a monitoring system capable of discovering and collecting metrics from workloads that expose them in a standard format
// (e.g. Prometheus exposition format). This ensures easy integration for collecting key metrics from common AI frameworks and servers.
var _ = WGDescribe("AI Service Metrics", func() {
	f := framework.NewDefaultFramework("ai-service-metrics")
	f.NamespacePodSecurityLevel = admissionapi.LevelBaseline
	const timeToWait = 15 * time.Minute

	ginkgo.BeforeEach(func(ctx context.Context) {
		// Check if Prometheus Operator is installed by trying to get its API resources.
		frameworkutil.SkipIfGroupVersionUnavaliable(ctx, f.ClientSet.Discovery(), "monitoring.coreos.com/v1")
	})

	/*
		Release: v1.34
		Testname: AI Service Metrics
		Description: Create a Deployment and exposes a custom metric via a ServiceMonitor. Query the prometheus
		and verify that the metric MUST be collected.
	*/
	frameworkutil.AIConformanceIt("metrics should be collected from the AI service", func(ctx context.Context) {
		ns := f.Namespace.Name
		name := "ai-service-metrics"
		metricName := "e2e:custom_metric"

		ginkgo.By("Getting the Prometheus instance")
		promOpClient, err := monitoring.NewForConfig(f.ClientConfig())
		framework.ExpectNoError(err, "error when creating prometheus operator client")
		promList, err := promOpClient.MonitoringV1().Prometheuses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		framework.ExpectNoError(err, "error when getting Prometheus list")
		gomega.Expect(promList.Items).ToNot(gomega.BeEmpty(), "at least one Prometheus should be found")
		prom := promList.Items[0]

		ginkgo.By("Create a resource consumer and initialize the custom metric value")
		rc := e2eautoscaling.NewDynamicResourceConsumer(ctx, name, ns, e2eautoscaling.KindDeployment, 1, 0, 0,
			150, 0, 0, metricName, f.ClientSet, f.ScalesGetter, e2eautoscaling.Disable, e2eautoscaling.Idle, nil)
		ginkgo.DeferCleanup(rc.CleanUp)

		ginkgo.By("Create a service monitor")
		sm := prometheusutil.CreateServiceMonitor(ctx, promOpClient, prom, f.ClientSet, ns, name, map[string]string{"name": name}, "http")
		framework.ExpectNoError(err, "error when creating service monitor")
		ginkgo.DeferCleanup(promOpClient.MonitoringV1().ServiceMonitors(sm.Namespace).Delete, sm.Name, metav1.DeleteOptions{})

		ginkgo.By("Wait for the metrics to be collected")
		query := fmt.Sprintf(`count by (__name__) ({job="%s", namespace="%s"})`, name, ns)
		err = framework.Gomega().Eventually(ctx, func(ctx context.Context) error {
			proxyRequest, err := e2eservice.GetServicesProxyRequest(f.ClientSet, f.ClientSet.CoreV1().RESTClient().Get())
			if err != nil {
				return err
			}
			req := proxyRequest.Namespace(prom.Namespace).
				Name(fmt.Sprintf("%s:http-web", prom.Name)).
				Suffix("/api/v1/query").
				Param("query", query)
			framework.Logf("Query URL: %v", *req.URL())
			data, err := req.DoRaw(ctx)
			if err != nil {
				return err
			}
			framework.Logf("Query result: %s", string(data))
			if !strings.Contains(string(data), "e2e:custom_metric") {
				return fmt.Errorf("metric %q not found: %s", metricName, string(data))
			}
			return nil
		}).WithTimeout(timeToWait).WithPolling(15 * time.Second).Should(gomega.Succeed())
		framework.ExpectNoError(err, "error when waiting for the metrics to be collected")
	})
})
