package pkg

import (
	"context"
	"os"
	"testing"

	plugin_helper "github.com/vmware-tanzu/sonobuoy-plugins/plugin-helper"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

func init() {
	utilruntime.Must(kueuev1beta1.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(clientgoscheme.Scheme))
}

var testenv env.Environment

func TestMain(m *testing.M) {
	cfg, _ := envconf.NewFromFlags()
	testenv = env.NewWithConfig(cfg)
	namespace := "cncf-ai-conformance"

	testenv.Setup(
		// Create the namespace for the tests to run in.
		envfuncs.CreateNamespace(namespace),
	)
	testenv.Finish(
		// Delete the namespace for the tests to run in.
		envfuncs.DeleteNamespace(namespace),
	)

	updateReporter := plugin_helper.NewProgressReporter(0)
	testenv.BeforeEachTest(func(ctx context.Context, cfg *envconf.Config, t *testing.T) (context.Context, error) {
		updateReporter.StartTest(t.Name())
		return ctx, nil
	})
	testenv.AfterEachTest(func(ctx context.Context, cfg *envconf.Config, t *testing.T) (context.Context, error) {
		updateReporter.StopTest(t.Name(), t.Failed(), t.Skipped(), nil)
		return ctx, nil
	})

	/*
		testenv.BeforeEachFeature(func(ctx context.Context, config *envconf.Config, info features.Feature) (context.Context, error) {
			// Note that you can also add logic here for before a feature is tested. There may be
			// more than one feature in a test.
			return ctx, nil
		})
		testenv.AfterEachFeature(func(ctx context.Context, config *envconf.Config, info features.Feature) (context.Context, error) {
			// Note that you can also add logic here for after a feature is tested. There may be
			// more than one feature in a test.
			return ctx, nil
		})
	*/

	os.Exit(testenv.Run(m))
}
