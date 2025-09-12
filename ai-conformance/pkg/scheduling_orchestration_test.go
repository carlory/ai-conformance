package pkg

import (
	"context"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestGangScheduling(t *testing.T) {
	description := "The platform must allow for the installation and successful operation of at least one gang scheduling solution " +
		"that ensures all-or-nothing scheduling for distributed AI workloads (e.g. Kueue, Volcano, etc.) To be conformant, " +
		"the vendor must demonstrate that their platform can successfully run at least one such solution."

	f := features.New("gang_scheduling").
		WithLabel("type", "scheduling_orchestration").
		WithLabel("id", "gang_scheduling").
		WithLabel("description", description).
		WithLabel("level", "MUST").
		Assess(description, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// TODO: Implement gang scheduling
			return ctx
		})

	testenv.Test(t, f.Feature())
}

// How we might test it: Prepare a node pool with N nodes, configured with a specific accelerator type,
// with min node pool size of N and max size of at least N+1. Assuming 1 accelerator A per node N,
// Create (A*N)+1 Pods, each requesting one accelerator resource from that pool, verify that at least
// one Pod is unschedulable (Pending), and the cluster autoscaler will increase the node count to N+1,
// causing the Pod to be Running. Delete that Pod, then the cluster autoscaler will remove the idle
// ccelerator node, returning the node count to N.
//
// See https://docs.google.com/document/d/1hXoSdh9FEs13Yde8DivCYjjXyxa7j4J8erjZPEGWuzc/edit?tab=t.0
func TestClusterAutoscaling(t *testing.T) {
	description := "If the platform provides a cluster autoscaler or an equivalent mechanism, it must be able to scale up/down node groups " +
		"containing specific accelerator types based on pending pods requesting those accelerators."

	f := features.New("cluster_autoscaling").
		WithLabel("type", "scheduling_orchestration").
		WithLabel("id", "cluster_autoscaling").
		Assess(description, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {

			return ctx
		})

	testenv.Test(t, f.Feature())
}

// How we might test it: A custom metrics pipeline is configured to expose accelerator-related
// custom metrics to the HPA. Create a Deployment with each Pod requests an accelerator and
// exposes a custom metric. Create an HorizontalPodAutoscaler targeting the Deployment.
// Introduce load to the sample application, causing the average custom metric value to significantly
// exceed the target, triggering a scale up. Then remove the load to trigger a scale down.
//
// See https://docs.google.com/document/d/1hXoSdh9FEs13Yde8DivCYjjXyxa7j4J8erjZPEGWuzc/edit?tab=t.0
func TestPodAutoscaling(t *testing.T) {
	description := "If the platform supports the HorizontalPodAutoscaler, it must function correctly for pods utilizing accelerators. " +
		"This includes the ability to scale these Pods based on custom metrics relevant to AI/ML workloads."

	f := features.New("pod_autoscaling").
		WithLabel("type", "scheduling_orchestration").
		WithLabel("id", "pod_autoscaling").
		Assess(description, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return ctx
		})

	testenv.Test(t, f.Feature())
}
