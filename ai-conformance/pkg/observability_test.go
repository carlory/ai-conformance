package pkg

import (
	"context"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// How we might test it: Given a node with a supported accelerator type, identify the Prometheus-compatible
// metrics endpoint for the accelerators on the node and scrape metrics from the endpoint. Parse the scraped
// metrics to find metrics for each supported accelerator on the node, including: accelerator utilization,
// memory usage, temperature, power usage, etc. The test can evolve to check for specific metric names once
// those are standardized.
//
// See https://docs.google.com/document/d/1hXoSdh9FEs13Yde8DivCYjjXyxa7j4J8erjZPEGWuzc/edit?tab=t.0
func TestAcceleratorMmetrics(t *testing.T) {
	description := "For supported accelerator types, expose fine-grained performance metrics via a standardized, " +
		"machine-readable format metrics endpoint. This must include a core set of metrics for per-accelerator utilization and memory usage." +
		"Additionally, other relevant metrics such as temperature, power draw, and interconnect bandwidth should be exposed if the underlying " +
		"hardware or virtualization layer makes them available. The list of metrics should align with emerging standards, such as OpenTelemetry " +
		"metrics, to ensure interoperability."

	f := features.New("accelerator_metrics").
		WithLabel("type", "observability").
		WithLabel("id", "accelerator_metrics").
		WithLabel("description", description).
		WithLabel("level", "MUST").
		Assess(description, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return ctx
		})

	testenv.Test(t, f.Feature())
}

// Because all these common metrics are exposed in the Prometheus format, the test verifies
// the platform’s monitoring system can collect Prometheus metrics. First deploy an AI application
// using a common framework, configure metrics collection for this application, generate sample
// traffic to the application, then queries the platform's monitoring system and verifies that
// key metrics from the AI application have been collected.
//
// See https://docs.google.com/document/d/1hXoSdh9FEs13Yde8DivCYjjXyxa7j4J8erjZPEGWuzc/edit?tab=t.0
func TestAcceleratorMetrics(t *testing.T) {
	description := "Provide a monitoring system capable of discovering and collecting metrics from workloads that expose them in a standard format " +
		"(e.g. Prometheus exposition format). This ensures easy integration for collecting key metrics from common AI frameworks and servers."

	f := features.New("ai_service_metrics").
		WithLabel("type", "observability").
		WithLabel("id", "ai_service_metrics").
		Assess(description, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return ctx
		})

	testenv.Test(t, f.Feature())
}
