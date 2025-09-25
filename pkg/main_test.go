package pkg

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"

	plugin_helper "github.com/vmware-tanzu/sonobuoy-plugins/plugin-helper"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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
	if v := flag.Lookup("kubeconfig"); v != nil {
		// the kubeconfig flag will be already registered once kueue or karpenter is imported, so we need to set it manually.
		kubeconfig := v.Value.String()
		cfg.WithKubeconfigFile(kubeconfig)
		if kubeconfig == "" {
			// A workaround to use "sigs.k8s.io/e2e-framework/third_party/helm"
			if _, ok := os.LookupEnv("KUBERNETES_SERVICE_HOST"); ok {
				err := WriteInClusterKubeconfig("/tmp/kubeconfig")
				if err != nil {
					log.Fatalf("error when writing kubeconfig: %v", err)
				}
				cfg.WithKubeconfigFile("/tmp/kubeconfig")
			} else {
				log.Fatalf("kubeconfig is not set")
			}
		}
	}
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

// WriteInClusterKubeconfig generates a kubeconfig file based on the in-cluster configuration.
// The generated file can be used by kubectl or other clients inside the Pod.
func WriteInClusterKubeconfig(path string) error {
	// Load the in-cluster configuration from service account and environment variables
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	// Create a new kubeconfig structure
	kubeconfig := clientcmdapi.NewConfig()

	// Define the cluster (API server endpoint + CA)
	cluster := clientcmdapi.NewCluster()
	cluster.Server = cfg.Host
	cluster.CertificateAuthority = cfg.CAFile
	kubeconfig.Clusters["in-cluster"] = cluster

	// Define the user (use the service account token)
	auth := clientcmdapi.NewAuthInfo()
	auth.Token = string(cfg.BearerToken)
	kubeconfig.AuthInfos["sa-user"] = auth

	// Define the context (link cluster and user)
	context := clientcmdapi.NewContext()
	context.Cluster = "in-cluster"
	context.AuthInfo = "sa-user"
	kubeconfig.Contexts["in-cluster"] = context
	kubeconfig.CurrentContext = "in-cluster"

	// Serialize the kubeconfig into YAML format
	data, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return err
	}

	// Write the kubeconfig YAML to the given file path
	return os.WriteFile(path, data, 0600)
}
