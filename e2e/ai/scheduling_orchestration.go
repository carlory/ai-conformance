package ai

import (
	"context"
	"encoding/json"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/samber/lo"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/test/e2e/framework"
	e2econfig "k8s.io/kubernetes/test/e2e/framework/config"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
	e2eautoscaling "github.com/carlory/ai-conformance/e2e/util/framework/autoscaling"
)

var _ = WGDescribe("Gang Scheduling", func() {
	f := framework.NewDefaultFramework("gang-autoscaling")
	f.NamespacePodSecurityLevel = admissionapi.LevelRestricted

	framework.Context("kueue", func() {
		ginkgo.BeforeEach(func(ctx context.Context) {
			frameworkutil.SkipIfGroupVersionUnavaliable(ctx, f.ClientSet.Discovery(), "kueue.x-k8s.io/v1beta1")
		})

		/*
			Release: v1.34
			Testname: Gang Scheduling with Kueue and Job workload
			Description: Create two jobs with the same template and each replica requests 1 Nvidia GPU. Also, pay attention
			to configure the parallelism and completions to be the same as the jobSize, which is 80% of the total avaliable GPUs
			per job. In this scenario there is not enough resources to run all pods for both jobs at the same time, but all jobs
			MUST be scheduled and succeed eventually.
		*/
		frameworkutil.AIConformanceIt("2 jobs should be scheduled and succeed one by one when there are not enough resources", func(ctx context.Context) {
			// TODO: implement this test
		})
	})
})

var _ = WGDescribe("Cluster Autoscaling", func() {
	f := framework.NewDefaultFramework("cluster-autoscaling")
	f.NamespacePodSecurityLevel = admissionapi.LevelRestricted

	ginkgo.BeforeEach(func(ctx context.Context) {
		frameworkutil.SkipUnlessClusterAutoscalerExists(ctx, f.ClientSet)
	})

	/*
		Release: v1.34
		Testname: Cluster Autoscaling
		Description: Create N pods requesting an accelerator via resource limits until the last one is pending and marked
		as unschedulable. The cluster autoscaler MUST provision an suitable node for the pending pod. Check the pod status
		becomes Running. Delete the pod and verify the node MUST be reclaimed within 15 minutes.
	*/
	frameworkutil.AIConformanceIt("should provision an suitable node for a pending pod requesting an accelerator via resource limits", func(ctx context.Context) {
		ns := f.Namespace.Name
		client := f.ClientSet

		ginkgo.By("Getting the current node names")
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		framework.ExpectNoError(err, "Failed to get node list")
		nodeNames := lo.Map(nodes.Items, func(node corev1.Node, _ int) string { return node.Name })
		framework.Logf("current node names: %v", nodeNames)

		ginkgo.By("Creating N pods requesting an accelerator until the last one is pending and marked as unschedulable")
		var pendingPod *corev1.Pod
		for pendingPod == nil {
			pod := e2epod.MakePod(ns, nil, nil, f.NamespacePodSecurityLevel, "")
			pod.Spec.Containers[0].Resources.Limits = map[corev1.ResourceName]resource.Quantity{
				// TODO: make it configurable
				corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
			}
			pod, err = client.CoreV1().Pods(f.Namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
			framework.ExpectNoError(err, "Failed to create pod")
			ginkgo.DeferCleanup(client.CoreV1().Pods(ns).Delete, pod.Name, metav1.DeleteOptions{})
			err = e2epod.WaitForPodCondition(ctx, client, ns, pod.Name, "PodScheduled", f.Timeouts.PodStartShort, func(pod *corev1.Pod) (bool, error) {
				if pod.Status.Phase == corev1.PodPending {
					for _, cond := range pod.Status.Conditions {
						if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse && cond.Reason == corev1.PodReasonUnschedulable {
							pendingPod = pod
							return true, nil
						}
						if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionTrue {
							return true, nil
						}
					}
				}
				return false, nil
			})
			framework.ExpectNoError(err, "error when getting the scheduling status of pod %s", pod.Name)
		}
		framework.Logf("the pending pod is made: %s", pendingPod.Name)

		ginkgo.By("Waiting for the pending pod to be running and not scheduled on an existing node")
		err = e2epod.WaitForPodRunningInNamespaceSlow(ctx, client, ns, pendingPod.Name)
		framework.ExpectNoError(err, "error when waiting for the pod %s to be running", pendingPod.Name)
		pod, err := client.CoreV1().Pods(ns).Get(ctx, pendingPod.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "error when retrieving the pod %s", pendingPod.Name)
		nodeName := pod.Spec.NodeName
		gomega.Expect(nodeName).ToNot(gomega.BeElementOf(nodeNames), "The pod should not be scheduled on an existing node")

		ginkgo.By("Deleting the pending pod and waiting for the node to be reclaimed")
		err = client.CoreV1().Pods(ns).Delete(ctx, pendingPod.Name, metav1.DeleteOptions{})
		framework.ExpectNoError(err, "error when deleting the pod %s", pendingPod.Name)
		err = e2epod.WaitForPodNotFoundInNamespace(ctx, client, pendingPod.Name, ns, f.Timeouts.PodStartShort)
		framework.ExpectNoError(err, "error when waiting for the pod %s to be deleted", pendingPod.Name)
		err = framework.Gomega().Eventually(ctx, framework.HandleRetry(func(ctx context.Context) (*corev1.Node, error) {
			node, err := f.ClientSet.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return node, err
		})).WithTimeout(15 * time.Minute).Should(gomega.BeNil())
		framework.ExpectNoError(err, "error when waiting for the node %s to be reclaimed", nodeName)
	})
})

var podAutoscaling struct {
	MetricName string `default:"" usage:"metric name to use for the HorizontalPodAutoscaler"`
}
var _ = e2econfig.AddOptions(&podAutoscaling, "ai.podAutoscaling")

var _ = WGDescribe("Pod Autoscaling", func() {
	f := framework.NewDefaultFramework("pod-autoscaling")
	f.NamespacePodSecurityLevel = admissionapi.LevelBaseline
	const timeToWait = 15 * time.Minute

	ginkgo.BeforeEach(func(ctx context.Context) {
		aggrclient, err := aggregatorclient.NewForConfig(f.ClientConfig())
		framework.ExpectNoError(err, "error when creating aggregator client")
		_, err = aggrclient.ApiregistrationV1().APIServices().Get(ctx, "v1beta1.custom.metrics.k8s.io", metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2eskipper.Skipf("The APIService v1beta1.custom.metrics.k8s.io does not exist")
			}
			framework.Failf("error when getting APIService v1beta1.custom.metrics.k8s.io: %v", err)
		}

		// Check if Prometheus Operator is installed by trying to get its API resources.
		frameworkutil.SkipIfGroupVersionUnavaliable(ctx, f.ClientSet.Discovery(), "monitoring.coreos.com/v1")
	})

	/*
		Release: v1.34
		Testname: Pod Autoscaling
		Description: Create a Deployment and exposes a custom metric. Create an HorizontalPodAutoscaler targeting the Deployment.
		Introduce load to the sample application, causing the average custom metric value to significantly exceed the target,
		triggering a scale up. Then remove the load to trigger a scale down.
	*/
	frameworkutil.AIConformanceIt("should scale up and down the workload based on the custom metrics", func(ctx context.Context) {
		ns := f.Namespace.Name
		replicas := 1
		minReplicas := 1
		maxReplicas := 2
		fristScale := 2
		secondScale := 1
		initCustomMetric := 150
		metricTargetValue := 50
		metricTargetType := autoscalingv2.AverageValueMetricType
		metricName := podAutoscaling.MetricName
		kind := e2eautoscaling.KindDeployment
		name := "resource-consumer"

		promOpClient, err := monitoring.NewForConfig(f.ClientConfig())
		framework.ExpectNoError(err, "error when creating prometheus operator client")
		promList, err := promOpClient.MonitoringV1().Prometheuses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		framework.ExpectNoError(err, "error when getting Prometheus list")
		gomega.Expect(promList.Items).ToNot(gomega.BeEmpty(), "at least one Prometheus should be found")
		prom := promList.Items[0]

		labels, err := metav1.LabelSelectorAsMap(prom.Spec.ServiceMonitorSelector)
		framework.ExpectNoError(err, "error when converting label selector to map")

		serviceMonitorNamespace := ns
		serviceMonitorNamespaceSelector := prom.Spec.ServiceMonitorNamespaceSelector
		if serviceMonitorNamespaceSelector != nil {
			nsLabels, err := metav1.LabelSelectorAsMap(prom.Spec.ServiceMonitorNamespaceSelector)
			framework.ExpectNoError(err, "error when converting label selector to map")

			if len(nsLabels) > 0 {
				nsPatch, err := json.Marshal(map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": nsLabels,
					},
				})
				framework.ExpectNoError(err, "error marshaling namespace patch")
				_, err = f.ClientSet.CoreV1().Namespaces().Patch(ctx, ns, types.StrategicMergePatchType, nsPatch, metav1.PatchOptions{})
				framework.ExpectNoError(err, "error patching namespace")
			}
		} else {
			serviceMonitorNamespace = prom.Namespace
		}

		ginkgo.By("Create a ServiceMonitor")
		serviceMonitor := &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: serviceMonitorNamespace,
				Labels:    labels,
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				NamespaceSelector: monitoringv1.NamespaceSelector{
					MatchNames: []string{ns},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"name": name},
				},
				Endpoints: []monitoringv1.Endpoint{
					{
						Port:     "http",
						Interval: "15s",
						Path:     "/metrics",
					},
				},
			},
		}
		_, err = promOpClient.MonitoringV1().ServiceMonitors(serviceMonitorNamespace).Create(ctx, serviceMonitor, metav1.CreateOptions{})
		framework.ExpectNoError(err, "error when creating service monitor")
		ginkgo.DeferCleanup(promOpClient.MonitoringV1().ServiceMonitors(serviceMonitorNamespace).Delete, serviceMonitor.Name, metav1.DeleteOptions{})

		ginkgo.By("Create a resource consumer and initialize the custom metric value to 150")
		rc := e2eautoscaling.NewDynamicResourceConsumer(ctx, name, ns, kind, replicas, 0, 0,
			initCustomMetric, 0, 0, metricName, f.ClientSet, f.ScalesGetter, e2eautoscaling.Disable, e2eautoscaling.Idle, nil)
		ginkgo.DeferCleanup(rc.CleanUp)

		ginkgo.By("Create an HorizontalPodAutoscaler")
		hpa := e2eautoscaling.CreatePodsHorizontalPodAutoscaler(ctx, rc, ns, metricName, metricTargetType, int32(metricTargetValue), int32(minReplicas), int32(maxReplicas))
		ginkgo.DeferCleanup(e2eautoscaling.DeleteHorizontalPodAutoscaler, rc, hpa.Name)

		ginkgo.By("Wait for the workload to be scaled up")
		rc.WaitForReplicas(ctx, fristScale, timeToWait)

		rc.Pause()
		ginkgo.By("Wait for the workload to be scaled down")
		rc.WaitForReplicas(ctx, secondScale, timeToWait)
	})
})
