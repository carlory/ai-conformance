package pkg

import (
	"context"
	"encoding/json"
	"testing"

	"k8s.io/client-go/discovery"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// How we might test it: Verify that all the resource.k8s.io/v1 DRA API resources are enabled.
//
// See https://docs.google.com/document/d/1hXoSdh9FEs13Yde8DivCYjjXyxa7j4J8erjZPEGWuzc/edit?tab=t.0
func TestDraSupport(t *testing.T) {
	description := "Support Dynamic Resource Allocation (DRA) APIs to enable more flexible and fine-grained resource requests beyond simple counts."

	f := features.New("dra_support").
		WithLabel("type", "accelerators").
		WithLabel("id", "dra_support").
		WithLabel("description", description).
		WithLabel("level", "MUST").
		Assess("Verify that all the resource.k8s.io/v1 DRA API resources are enabled.", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatalf("Failed to create discovery client: %v", err)
			}
			resources, err := discoveryClient.ServerResourcesForGroupVersion("resource.k8s.io/v1")
			if err != nil {
				t.Fatalf("the resource.k8s.io/v1 is not enabled: %v", err)
			}
			if resources == nil {
				t.Fatalf("no resources found in resource.k8s.io/v1")
			} else {
				data, _ := json.Marshal(resources)
				t.Logf("found resources in resource.k8s.io/v1: %s", string(data))
			}
			return ctx
		})

	testenv.Test(t, f.Feature())
}
