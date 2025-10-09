package ai

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/samber/lo"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	resourcehelper "k8s.io/component-helpers/resource"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/test/e2e/framework"
	e2econfig "k8s.io/kubernetes/test/e2e/framework/config"
	e2egpu "k8s.io/kubernetes/test/e2e/framework/gpu"
	e2ejob "k8s.io/kubernetes/test/e2e/framework/job"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	kueueclient "sigs.k8s.io/kueue/client-go/clientset/versioned"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
	e2eautoscaling "github.com/carlory/ai-conformance/e2e/util/framework/autoscaling"
	prometheusutil "github.com/carlory/ai-conformance/e2e/util/prometheus"
)

var _ = WGDescribe("Gang Scheduling", func() {
	f := framework.NewDefaultFramework("gang-autoscaling")
	f.NamespacePodSecurityLevel = admissionapi.LevelBaseline
	var ns string
	var avaliableGPUs int

	ginkgo.BeforeEach(func(ctx context.Context) {
		ns = f.Namespace.Name

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

		used := 0
		pods, err := f.ClientSet.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		framework.ExpectNoError(err)
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				continue
			}
			for resource, val := range resourcehelper.PodLimits(&pod, resourcehelper.PodResourcesOptions{}) {
				if string(resource) == e2egpu.NVIDIAGPUResourceName {
					used += int(val.Value())
				}
			}
		}

		avaliableGPUs = allocatable - used
		if avaliableGPUs < 2 {
			e2eskipper.Skipf("At least 2 Nvidia GPU(s) are required. Only %d/%d are available", avaliableGPUs, allocatable)
		}
	})

	framework.Context("kueue", func() {
		var kueueClient kueueclient.Interface
		var err error
		ginkgo.BeforeEach(func(ctx context.Context) {
			frameworkutil.SkipIfGroupVersionUnavaliable(ctx, f.ClientSet.Discovery(), "kueue.x-k8s.io/v1beta1")
			kueueClient, err = kueueclient.NewForConfig(f.ClientConfig())
			framework.ExpectNoError(err, "error when creating kueue client")
		})

		/*
			Release: v1.33
			Testname: Gang Scheduling with Kueue and Job workload
			Description: Create two jobs with the same template and each replica requests 1 Nvidia GPU. Also, pay attention
			to configure the parallelism and completions to be the same as the jobSize, which is 80% of the total avaliable GPUs
			per job. In this scenario there is not enough resources to run all pods for both jobs at the same time, but all jobs
			MUST be scheduled and succeed eventually.
		*/
		frameworkutil.AIConformanceIt("2 jobs should be scheduled and succeed one by one when there are not enough resources", framework.WithSerial(), func(ctx context.Context) {
			// We configure the gpu flavor by doubling the total gpu allocatable in our cluster,
			// in order to simulate the deadlock scenario with provisioning when kueue doesn't enable
			// the waitForPodsReady feature which is documented in this link:
			// https://kueue.sigs.k8s.io/docs/tasks/manage/setup_wait_for_pods_ready/
			nominalQuota := avaliableGPUs * 2

			// We create two jobs with the same template and each replica requests 1 Nvidia GPU. Also, pay attention
			// to configure the parallelism and completions to be the same as the jobSize, which is 80% of the total
			// avaliable GPUs per job.
			// In this scenario there is not enough resources to run all pods for both jobs at the same time, risking
			// deadlock.
			jobSize := int32(math.Ceil(float64(avaliableGPUs) * 0.8))

			ginkgo.By("Creating a resource flavor")
			rf := &kueuev1beta1.ResourceFlavor{ObjectMeta: metav1.ObjectMeta{Name: f.UniqueName}}
			_, err = kueueClient.KueueV1beta1().ResourceFlavors().Create(ctx, rf, metav1.CreateOptions{})
			framework.ExpectNoError(err, "error when creating resource flavor")
			ginkgo.DeferCleanup(kueueClient.KueueV1beta1().ResourceFlavors().Delete, rf.Name, metav1.DeleteOptions{})

			ginkgo.By("Creating a cluster queue")
			clusterQueue := &kueuev1beta1.ClusterQueue{
				ObjectMeta: metav1.ObjectMeta{Name: f.UniqueName},
				Spec: kueuev1beta1.ClusterQueueSpec{
					NamespaceSelector: &metav1.LabelSelector{},
					ResourceGroups: []kueuev1beta1.ResourceGroup{
						{
							CoveredResources: []corev1.ResourceName{e2egpu.NVIDIAGPUResourceName},
							Flavors: []kueuev1beta1.FlavorQuotas{
								{
									Name: kueuev1beta1.ResourceFlavorReference(rf.Name),
									Resources: []kueuev1beta1.ResourceQuota{
										{
											Name:         e2egpu.NVIDIAGPUResourceName,
											NominalQuota: resource.MustParse(strconv.Itoa(nominalQuota)),
										},
									},
								},
							},
						},
					},
				},
			}
			_, err = kueueClient.KueueV1beta1().ClusterQueues().Create(ctx, clusterQueue, metav1.CreateOptions{})
			framework.ExpectNoError(err, "error when creating cluster queue")
			ginkgo.DeferCleanup(kueueClient.KueueV1beta1().ClusterQueues().Delete, clusterQueue.Name, metav1.DeleteOptions{})

			ginkgo.By("Creating a local queue")
			localQueue := &kueuev1beta1.LocalQueue{
				ObjectMeta: metav1.ObjectMeta{Name: f.UniqueName},
				Spec: kueuev1beta1.LocalQueueSpec{
					ClusterQueue: kueuev1beta1.ClusterQueueReference(clusterQueue.Name),
				},
			}
			_, err = kueueClient.KueueV1beta1().LocalQueues(ns).Create(ctx, localQueue, metav1.CreateOptions{})
			framework.ExpectNoError(err, "error when creating local queue")
			ginkgo.DeferCleanup(kueueClient.KueueV1beta1().LocalQueues(ns).Delete, localQueue.Name, metav1.DeleteOptions{})

			ginkgo.By("Creating 2 jobs with the same template but different names and wait for them to complete")
			wg := sync.WaitGroup{}
			for _, jobName := range []string{"job1", "job2"} {
				wg.Add(1)
				go func(jobName string) {
					defer ginkgo.GinkgoRecover()
					defer wg.Done()
					createJobForGangScheduling(ctx, f.ClientSet, ns, jobName, jobSize, localQueue.Name)
					err = e2ejob.WaitForJobComplete(ctx, f.ClientSet, f.Namespace.Name, jobName, batchv1.JobReasonCompletionsReached, jobSize)
					framework.ExpectNoError(err, "failed to ensure that job %s completed", jobName)
				}(jobName)
			}
			wg.Wait()
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
		Release: v1.33
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
				corev1.ResourceName(e2egpu.NVIDIAGPUResourceName): resource.MustParse("1"),
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
		Release: v1.33
		Testname: Pod Autoscaling
		Description: Create a Deployment and exposes a custom metric via a ServiceMonitor. Create an HorizontalPodAutoscaler
		targeting the Deployment. Introduce load to the sample application, causing the average custom metric value to
		significantly exceed the target, triggering a scale up. Then remove the load to trigger a scale down.
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

		ginkgo.By("Getting the Prometheus instance")
		promOpClient, err := monitoring.NewForConfig(f.ClientConfig())
		framework.ExpectNoError(err, "error when creating prometheus operator client")
		promList, err := promOpClient.MonitoringV1().Prometheuses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		framework.ExpectNoError(err, "error when getting Prometheus list")
		gomega.Expect(promList.Items).ToNot(gomega.BeEmpty(), "at least one Prometheus should be found")
		prom := promList.Items[0]

		ginkgo.By("Create a resource consumer and initialize the custom metric value")
		rc := e2eautoscaling.NewDynamicResourceConsumer(ctx, name, ns, kind, replicas, 0, 0,
			initCustomMetric, 0, 0, metricName, f.ClientSet, f.ScalesGetter, e2eautoscaling.Disable, e2eautoscaling.Idle, nil)
		ginkgo.DeferCleanup(rc.CleanUp)

		ginkgo.By("Create a service monitor")
		framework.ExpectNoError(err, "error when creating prometheus operator client")
		sm := prometheusutil.CreateServiceMonitor(ctx, promOpClient, prom, f.ClientSet, ns, name, map[string]string{"name": name}, "http")
		framework.ExpectNoError(err, "error when creating service monitor")
		ginkgo.DeferCleanup(promOpClient.MonitoringV1().ServiceMonitors(sm.Namespace).Delete, sm.Name, metav1.DeleteOptions{})

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

func createJobForGangScheduling(ctx context.Context, client clientset.Interface, ns string, name string, jobSize int32, queueName string) {
	labels := map[string]string{"job": name}
	// Create a headless service for pod-to-pod communication
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: corev1.ProtocolTCP,
					Port:     8080,
				},
			},
		},
	}
	_, err := client.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	framework.ExpectNoError(err, "error when creating service")
	ginkgo.DeferCleanup(client.CoreV1().Services(ns).Delete, svc.Name, metav1.DeleteOptions{})

	// Create a config map to store the script code
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"main.py": fmt.Sprintf(`
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.request import urlopen
import sys, os, time, logging

logging.basicConfig(stream=sys.stdout, level=logging.DEBUG)
serverPort = 8080
INDEX_COUNT = int(sys.argv[1])
index = int(os.environ.get('JOB_COMPLETION_INDEX'))
logger = logging.getLogger('LOG' + str(index))

class WorkerServer(BaseHTTPRequestHandler):
	def do_GET(self):
		self.send_response(200)
		self.end_headers()
		if "exit" in self.path:
			self.wfile.write(bytes("Exiting", "utf-8"))
			self.wfile.close()
			sys.exit(0)
		else:
			self.wfile.write(bytes("Running", "utf-8"))

def call_until_success(url):
	while True:
		try:
			logger.info("Calling URL: " + url)
			with urlopen(url) as response:
				response_content = response.read().decode('utf-8')
				logger.info("Response content from %%s: %%s" %% (url, response_content))
				return
		except Exception as e:
			logger.warning("Got exception when calling %%s: %%s" %% (url, e))
		time.sleep(1)

if __name__ == "__main__":
	if index == 0:
		for i in range(1, INDEX_COUNT):
			call_until_success("http://%[1]s-%%d.%[2]s:8080/ping" %% i)
		logger.info("All workers running")

		time.sleep(10) # sleep 10s to simulate doing something

		for i in range(1, INDEX_COUNT):
			call_until_success("http://%[1]s-%%d.%[2]s:8080/exit" %% i)
		logger.info("All workers stopped")
	else:
		webServer = HTTPServer(("", serverPort), WorkerServer)
		logger.info("Server started at port %%s" %% serverPort)
		webServer.serve_forever()`, name, name),
		},
	}
	_, err = client.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
	framework.ExpectNoError(err, "error when creating config map")
	ginkgo.DeferCleanup(client.CoreV1().ConfigMaps(ns).Delete, cm.Name, metav1.DeleteOptions{})
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"kueue.x-k8s.io/queue-name": queueName,
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism:    &jobSize,
			Completions:    &jobSize,
			CompletionMode: ptr.To(batchv1.IndexedCompletion),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Subdomain:     name,
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "script-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: name,
									},
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Effect:   corev1.TaintEffectNoSchedule,
							Operator: corev1.TolerationOpExists,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "main",
							Image:           "docker.io/library/python:bullseye",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"python"},
							Args:            []string{"/script-path/main.py", strconv.Itoa(int(jobSize))},
							Ports:           []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}},
							VolumeMounts:    []corev1.VolumeMount{{Name: "script-volume", MountPath: "/script-path"}},
							Resources: corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceName(e2egpu.NVIDIAGPUResourceName): resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = client.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{})
	framework.ExpectNoError(err, "error when creating job")
	ginkgo.DeferCleanup(client.BatchV1().Jobs(ns).Delete, job.Name, metav1.DeleteOptions{})
}
