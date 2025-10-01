package ai

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	admissionapi "k8s.io/pod-security-admission/api"

	drautils "k8s.io/kubernetes/test/e2e/dra/utils"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
)

// https://github.com/kubernetes-sigs/wg-ai-conformance/issues/27#issuecomment-3356364245
// Remove it once the test is included in k/k conformance tests.
var _ = WGDescribe("Secure Accelerator Access", func() {
	f := framework.NewDefaultFramework("dra")

	// The driver containers have to run with sufficient privileges to
	// modify /var/lib/kubelet/plugins.
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	f.Context("kubelet", func() {
		nodes := drautils.NewNodes(f, 1, 1)
		driver := drautils.NewDriver(f, nodes, drautils.NetworkResources(10, false))
		b := drautils.NewBuilder(f, driver)

		frameworkutil.AIConformanceIt("registers plugin", func() {
			ginkgo.By("the driver is running")
		})

		frameworkutil.AIConformanceIt("must map configs and devices to the right containers", func(ctx context.Context) {
			// Several claims, each with three requests and three configs.
			// One config applies to all requests, the other two only to one request each.
			claimForAllContainers := b.ExternalClaim()
			claimForAllContainers.Name = "all"
			claimForAllContainers.Spec.Devices.Requests = append(claimForAllContainers.Spec.Devices.Requests,
				*claimForAllContainers.Spec.Devices.Requests[0].DeepCopy(),
				*claimForAllContainers.Spec.Devices.Requests[0].DeepCopy(),
			)
			claimForAllContainers.Spec.Devices.Requests[0].Name = "req0"
			claimForAllContainers.Spec.Devices.Requests[1].Name = "req1"
			claimForAllContainers.Spec.Devices.Requests[2].Name = "req2"
			claimForAllContainers.Spec.Devices.Config = append(claimForAllContainers.Spec.Devices.Config,
				*claimForAllContainers.Spec.Devices.Config[0].DeepCopy(),
				*claimForAllContainers.Spec.Devices.Config[0].DeepCopy(),
			)
			claimForAllContainers.Spec.Devices.Config[0].Requests = nil
			claimForAllContainers.Spec.Devices.Config[1].Requests = []string{"req1"}
			claimForAllContainers.Spec.Devices.Config[2].Requests = []string{"req2"}
			claimForAllContainers.Spec.Devices.Config[0].Opaque.Parameters.Raw = []byte(`{"all_config0":"true"}`)
			claimForAllContainers.Spec.Devices.Config[1].Opaque.Parameters.Raw = []byte(`{"all_config1":"true"}`)
			claimForAllContainers.Spec.Devices.Config[2].Opaque.Parameters.Raw = []byte(`{"all_config2":"true"}`)

			claimForContainer0 := claimForAllContainers.DeepCopy()
			claimForContainer0.Name = "container0"
			claimForContainer0.Spec.Devices.Config[0].Opaque.Parameters.Raw = []byte(`{"container0_config0":"true"}`)
			claimForContainer0.Spec.Devices.Config[1].Opaque.Parameters.Raw = []byte(`{"container0_config1":"true"}`)
			claimForContainer0.Spec.Devices.Config[2].Opaque.Parameters.Raw = []byte(`{"container0_config2":"true"}`)
			claimForContainer1 := claimForAllContainers.DeepCopy()
			claimForContainer1.Name = "container1"
			claimForContainer1.Spec.Devices.Config[0].Opaque.Parameters.Raw = []byte(`{"container1_config0":"true"}`)
			claimForContainer1.Spec.Devices.Config[1].Opaque.Parameters.Raw = []byte(`{"container1_config1":"true"}`)
			claimForContainer1.Spec.Devices.Config[2].Opaque.Parameters.Raw = []byte(`{"container1_config2":"true"}`)

			pod := b.PodExternal()
			pod.Spec.ResourceClaims = []v1.PodResourceClaim{
				{
					Name:              "all",
					ResourceClaimName: &claimForAllContainers.Name,
				},
				{
					Name:              "container0",
					ResourceClaimName: &claimForContainer0.Name,
				},
				{
					Name:              "container1",
					ResourceClaimName: &claimForContainer1.Name,
				},
			}

			// Add a second container.
			pod.Spec.Containers = append(pod.Spec.Containers, *pod.Spec.Containers[0].DeepCopy())
			pod.Spec.Containers[0].Name = "container0"
			pod.Spec.Containers[1].Name = "container1"

			// All claims use unique env variables which can be used to verify that they
			// have been mapped into the right containers. In addition, the test driver
			// also sets "claim_<claim name>_<request name>=true" with non-alphanumeric
			// replaced by underscore.

			// Both requests (claim_*_req*) and all user configs (user_*_config*).
			allContainersEnv := []string{
				"user_all_config0", "true",
				"user_all_config1", "true",
				"user_all_config2", "true",
				"claim_all_req0", "true",
				"claim_all_req1", "true",
				"claim_all_req2", "true",
			}

			// Everything from the "all" claim and everything from the "container0" claim.
			pod.Spec.Containers[0].Resources.Claims = []v1.ResourceClaim{{Name: "all"}, {Name: "container0"}}
			container0Env := []string{
				"user_container0_config0", "true",
				"user_container0_config1", "true",
				"user_container0_config2", "true",
				"claim_container0_req0", "true",
				"claim_container0_req1", "true",
				"claim_container0_req2", "true",
			}
			container0Env = append(container0Env, allContainersEnv...)

			// Everything from the "all" claim, but only the second request from the "container1" claim.
			// The first two configs apply.
			pod.Spec.Containers[1].Resources.Claims = []v1.ResourceClaim{{Name: "all"}, {Name: "container1", Request: "req1"}}
			container1Env := []string{
				"user_container1_config0", "true",
				"user_container1_config1", "true",
				// Does not apply: user_container1_config2
				"claim_container1_req1", "true",
			}
			container1Env = append(container1Env, allContainersEnv...)

			b.Create(ctx, claimForAllContainers, claimForContainer0, claimForContainer1, pod)
			err := e2epod.WaitForPodRunningInNamespace(ctx, f.ClientSet, pod)
			framework.ExpectNoError(err, "start pod")

			drautils.TestContainerEnv(ctx, f, pod, pod.Spec.Containers[0].Name, true, container0Env...)
			drautils.TestContainerEnv(ctx, f, pod, pod.Spec.Containers[1].Name, true, container1Env...)
		})
	})
})
