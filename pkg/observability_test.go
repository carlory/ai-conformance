package pkg

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
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
func TestAcceleratorMetrics(t *testing.T) {
	description := "For supported accelerator types, the platform must allow for the installation and successful operation of at least " +
		"one accelerator metrics solution that exposes fine-grained performance metrics via a standardized, machine-readable metrics endpoint. " +
		"This must include a core set of metrics for per-accelerator utilization and memory usage. Additionally, other relevant metrics such as " +
		"temperature, power draw, and interconnect bandwidth should be exposed if the underlying hardware or virtualization layer makes them available." +
		"The list of metrics should align with emerging standards, such as OpenTelemetry metrics, to ensure interoperability. " +
		"The platform may provide a managed solution, but this is not required for conformance."

	f := features.New("accelerator_metrics").
		WithLabel("type", "observability").
		WithLabel("id", "accelerator_metrics").
		WithLabel("level", "MUST").
		AssessWithDescription("Verify that the metrics are collected from the GPU node", description, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			jobName := "nvidia-dcgm-exporter"

			// TODO: use a better way to retrieve the metrics.
			t.Logf("Waiting for the metrics to be collected")
			time.Sleep(30 * time.Second)

			queryString := fmt.Sprintf(`count by (__name__) ({job="%s"})`, jobName)

			raw, err := QueryPrometheus(PrometheusQueryParams{
				Config:                cfg,
				PrometheusmNamespace:  "monitoring",
				PrometheusServiceName: "kube-prometheus-stack-prometheus",
				Query:                 queryString,
			})
			if err != nil {
				t.Errorf("error when getting prometheus query: %v", err)
				return ctx
			}
			t.Logf("prometheus query result: \n%s", string(raw))

			if raw == "" {
				t.Errorf("prometheus query result is empty")
				return ctx
			}
			// FIXME: I don't know which kind of metrics should be checked.
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
func TestAIServiceMetrics(t *testing.T) {
	description := "Provide a monitoring system capable of discovering and collecting metrics from workloads that expose them in a standard format " +
		"(e.g. Prometheus exposition format). This ensures easy integration for collecting key metrics from common AI frameworks and servers."

	type PrometheusJobNameContextKey struct{}
	f := features.New("ai_service_metrics").
		WithLabel("type", "observability").
		WithLabel("id", "ai_service_metrics").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			SkipIfGroupVersionUnavaliable(t, cfg, "monitoring.coreos.com/v1")

			var prometheusJobName string
			createFn := func(ctx context.Context, obj k8s.Object) error {
				t.Logf("Creating %s %s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
				if obj.GetObjectKind().GroupVersionKind().Kind == "ServiceMonitor" {
					prometheusJobName = obj.GetName()
				}
				return cfg.Client().Resources().Create(ctx, obj)
			}

			err := decoder.DecodeEachFile(ctx, fs.FS(os.DirFS("testdata")), "ai_service_metrics/*.yaml", createFn, decoder.MutateNamespace(cfg.Namespace()))
			if err != nil {
				t.Error(err)
				return ctx
			}

			// Wait for the pods to be created.
			time.Sleep(5 * time.Second)

			pods := &corev1.PodList{}
			err = cfg.Client().Resources(cfg.Namespace()).List(ctx, pods)
			if err != nil {
				t.Errorf("error when getting Pods in the namespace: %s: %v", cfg.Namespace(), err)
				return ctx
			}

			t.Logf("Waiting for %d Pods to be ready", len(pods.Items))
			err = wait.For(conditions.New(cfg.Client().Resources()).ResourcesMatch(pods, func(obj k8s.Object) bool {
				return podutils.IsPodReady(obj.(*corev1.Pod))
			}))
			if err != nil {
				t.Errorf("error when checking all Pods ready in the namespace %s: %v", cfg.Namespace(), err)
				return ctx
			}

			return context.WithValue(ctx, PrometheusJobNameContextKey{}, prometheusJobName)
		}).
		AssessWithDescription("Verify that the metrics are collected from the AI workload", description, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			jobName := ctx.Value(PrometheusJobNameContextKey{}).(string)

			// TODO: use a better way to retrieve the metrics.
			t.Logf("Waiting for the metrics to be collected")
			time.Sleep(time.Minute)

			queryString := fmt.Sprintf(`count by (__name__) ({job="%s", namespace="%s"})`, jobName, cfg.Namespace())

			raw, err := QueryPrometheus(PrometheusQueryParams{
				Config:                cfg,
				PrometheusmNamespace:  "monitoring",
				PrometheusServiceName: "kube-prometheus-stack-prometheus",
				Query:                 queryString,
			})
			if err != nil {
				t.Errorf("error when getting prometheus query: %v", err)
				return ctx
			}
			t.Logf("prometheus query result: \n%s", string(raw))

			if raw == "" {
				t.Errorf("prometheus query result is empty")
				return ctx
			}
			// FIXME: I don't know which kind of metrics should be checked.
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := decoder.DeleteWithManifestDir(ctx, cfg.Client().Resources(), "testdata", "ai_service_metrics/*.yaml", []resources.DeleteOption{}, decoder.MutateNamespace(cfg.Namespace()))
			if err != nil {
				t.Errorf("error when deleting the resources: %v", err)
			}
			return ctx
		})

	testenv.Test(t, f.Feature())
}
