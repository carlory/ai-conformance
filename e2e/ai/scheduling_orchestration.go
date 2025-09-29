package ai

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionapi "k8s.io/pod-security-admission/api"

	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
)

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
