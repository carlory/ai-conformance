package ai

import (
	"context"

	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
)

// Ensure that access to accelerators from within containers is properly isolated and mediated by
// the Kubernetes resource management framework (device plugin or DRA) and container runtime, preventing unauthorized access or interference between workloads.
var _ = WGDescribe("Secure Accelerator Access", func() {
	f := framework.NewDefaultFramework("secure-accelerator-access")
	f.NamespacePodSecurityLevel = admissionapi.LevelRestricted

	/*
		Release: v1.34
		Testname: Secure Accelerator Access
		Description: Deploy a Pod to a node with available accelerators, without requesting accelerator
		resources in the Pod spec. Execute a command in the Pod to probe for accelerator devices,
		and the command should fail or report that no accelerator devices are found.
	*/
	frameworkutil.AIConformanceIt("should fail or report that no accelerator devices are found", func(ctx context.Context) {
		// TODO: implement this test
	})

	/*
		Release: v1.34
		Testname: Secure Accelerator Access
		Description: Create two Pods, each is allocated an accelerator resource. Execute a command
		in one Pod to attempt to access the other Pod’s accelerator, and should be denied.
	*/
	frameworkutil.AIConformanceIt("should be denied to access the other Pod's accelerator", func(ctx context.Context) {
		// TODO: implement this test
	})
})
