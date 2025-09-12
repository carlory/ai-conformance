package pkg

import (
	"context"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// How we might test it: Deploy a representative AI operator, verify all Pods of the operator
// and its webhook are Running and its CRDs are registered with the API server. Verify that
// invalid attempts (e.g. invalid spec) should be rejected by its admission webhook. Verify
// that a valid instance of the custom resource can be reconciled.
//
// See https://docs.google.com/document/d/1hXoSdh9FEs13Yde8DivCYjjXyxa7j4J8erjZPEGWuzc/edit?tab=t.0
func TestRobustController(t *testing.T) {
	description := "The platform must prove that at least one complex AI operator with a CRD (e.g., Ray, Kubeflow) can be installed and functions reliably. " +
		"This includes verifying that the operator's pods run correctly, its webhooks are operational, and its custom resources can be reconciled."

	f := features.New("robust_controller").
		WithLabel("type", "operator").
		WithLabel("id", "robust_controller").
		WithLabel("description", description).
		WithLabel("level", "MUST").
		Assess(description, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return ctx
		})

	testenv.Test(t, f.Feature())
}
