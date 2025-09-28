package ai

import (
	"bytes"
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	clientset "k8s.io/client-go/kubernetes"

	"k8s.io/kubernetes/test/e2e/framework"
	e2econfig "k8s.io/kubernetes/test/e2e/framework/config"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	frameworkutil "github.com/carlory/ai-conformance/e2e/util/framework"
	e2ecrd "github.com/carlory/ai-conformance/e2e/util/framework/crd"
)

var operator struct {
	Filename    string `default:"" usage:"filename, directory, or URL to files to use to install the operator"`
	Chart       string `default:"" usage:"chart name where to locate the requested chart"`
	Repo        string `default:"" usage:"chart repository url where to locate the requested chart"`
	Namespace   string `default:"" usage:"namespace scope for this request. If unspecified, a random namespace will be used"`
	ReleaseName string `default:"" usage:"release name to create with this request. If unspecified, a random release name will be used"`
}

var _ = e2econfig.AddOptions(&operator, "ai.operator")

var _ = WGDescribe("Robust Controller", func() {
	f := framework.NewDefaultFramework("robust-controller")

	/*
		Release: v1.34
		Testname: Robust Controller
		Description: Deploy the given operator with filename or helm chart. All the pods of the operator MUST be
		running. If the operator has webhooks, all the pods of the webhooks MUST be running. The CRDs of the operator
		MUST have NamesAccepted and Established conditions with True status. And at least one CRD should have status
		or scale subresource to approve it can be reconciled by
	*/

	frameworkutil.AIConformanceIt("All pods of the operator and its webhooks should be running and its crds should be ready for use", func(ctx context.Context) {
		if operator.Chart != "" && operator.ReleaseName == "" {
			operator.ReleaseName = f.UniqueName
		}
		if operator.Namespace == "" {
			operator.Namespace = f.Namespace.Name
		}

		// Create a builder
		builder := resource.NewBuilder(frameworkutil.NewClientGetter(f)).
			Unstructured().
			// Accumulate as many items as possible
			ContinueOnError().
			// The namespace might not be populated to the generated manifests, so we need to set it manually.
			NamespaceParam(operator.Namespace).DefaultNamespace().
			// Flatten items contained in List objects
			Flatten()

		// set resource sources for the builder
		if operator.Chart != "" {
			// Provide the generated manifests via a Reader.
			manifests, err := frameworkutil.RunHelm(operator.Namespace, "template", operator.ReleaseName, operator.Chart, "--include-crds", "--repo", operator.Repo)
			framework.ExpectNoError(err)
			builder = builder.Stream(bytes.NewBufferString(manifests), operator.Chart)
			framework.Logf("generated manifests from chart %s with release name %s: %s", operator.Chart, operator.ReleaseName, manifests)
		}
		if operator.Filename != "" {
			// As an alternative, could call Path(false, "/path/to/file") to read from a file.
			builder = builder.FilenameParam(false, &resource.FilenameOptions{Filenames: []string{operator.Filename}})
		}

		// Run the builder and get the resource infos
		infos, err := builder.Do().Infos()
		framework.ExpectNoError(err)
		gomega.Expect(infos).ToNot(gomega.BeEmpty(), "at least one resource should be found from filename %s or chart %s", operator.Filename, operator.Chart)

		// Install the operator
		if operator.Filename != "" {
			_, err := e2ekubectl.RunKubectl(operator.Namespace, "apply", "-f", operator.Filename)
			ginkgo.DeferCleanup(e2ekubectl.RunKubectl, operator.Namespace, "delete", "-f", operator.Filename)
			framework.ExpectNoError(err, "error when applying operator from filename %s", operator.Filename)
		}
		if operator.Chart != "" {
			_, err := frameworkutil.RunHelm(operator.Namespace, "install", operator.ReleaseName, operator.Chart, "--create-namespace", "--debug", "--wait", "--timeout", "15m", "--repo", operator.Repo)
			ginkgo.DeferCleanup(frameworkutil.RunHelm, operator.Namespace, "uninstall", operator.ReleaseName, "--ignore-not-found")
			framework.ExpectNoError(err, "error when installing operator from chart %s with release name %s", operator.Chart, operator.ReleaseName)
		}

		// check installed resources
		var checkWebhookFn = func(ctx context.Context, client clientset.Interface, namespace, svcName string) error {
			svc, err := client.CoreV1().Services(namespace).Get(ctx, svcName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.FormatLabels(svc.Spec.Selector)})
			if err != nil {
				return err
			}
			if len(pods.Items) == 0 {
				return fmt.Errorf("at least one pod should be found for service %s in namespace %s", svcName, namespace)
			}
			for _, pod := range pods.Items {
				err := e2epod.WaitForPodNameRunningInNamespace(ctx, client, pod.Name, namespace)
				if err != nil {
					return err
				}
			}
			return nil
		}

		crds := []*apiextensionsv1.CustomResourceDefinition{}
		for _, info := range infos {
			obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(info.Object)
			if err != nil {
				framework.ExpectNoError(err, "error when converting object to unstructured: \n%s", format.Object(info.Object, 1))
			}

			switch info.Mapping.Resource {
			case apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"):
				crd := &apiextensionsv1.CustomResourceDefinition{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, crd)
				framework.ExpectNoError(err, "error when converting unstructured to %T: \n%s", crd, format.Object(obj, 1))
				crds = append(crds, crd)
				// check if the CRD is accepted and established
				apiExtensionClient, err := apiextclientset.NewForConfig(f.ClientConfig())
				framework.ExpectNoError(err, "error when creating api extension client")
				err = e2ecrd.WaitForCrdEstablishedAndNamesAccepted(ctx, apiExtensionClient, crd.GetName())
				framework.ExpectNoError(err, "error when waiting for CRD %s to be established and names accepted", crd.GetName())
				framework.Logf("CustomResourceDefinition %s is ready", crd.GetName())
			case admissionregistrationv1.SchemeGroupVersion.WithResource("validatingwebhookconfigurations"):
				config := &admissionregistrationv1.ValidatingWebhookConfiguration{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, config)
				framework.ExpectNoError(err, "error when converting unstructured to %T: \n%s", config, format.Object(obj, 1))
				for _, webhook := range config.Webhooks {
					if webhook.ClientConfig.Service != nil {
						svcName := webhook.ClientConfig.Service.Name
						svcNamespace := webhook.ClientConfig.Service.Namespace
						err := checkWebhookFn(ctx, f.ClientSet, svcNamespace, svcName)
						framework.ExpectNoError(err, "error when checking service %s/%s for ValidatingWebhookConfiguration %s", svcNamespace, svcName, config.GetName())
					}
				}
				framework.Logf("ValidatingWebhookConfiguration %s is ready", config.GetName())
			case admissionregistrationv1.SchemeGroupVersion.WithResource("mutatingwebhookconfigurations"):
				config := &admissionregistrationv1.MutatingWebhookConfiguration{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, config)
				framework.ExpectNoError(err, "error when converting unstructured to %T: \n%s", config, format.Object(obj, 1))
				for _, webhook := range config.Webhooks {
					if webhook.ClientConfig.Service != nil {
						svcName := webhook.ClientConfig.Service.Name
						svcNamespace := webhook.ClientConfig.Service.Namespace
						err := checkWebhookFn(ctx, f.ClientSet, svcNamespace, svcName)
						framework.ExpectNoError(err, "error when checking service %s/%s for MutatingWebhookConfiguration %s", svcNamespace, svcName, config.GetName())
					}
				}
				framework.Logf("MutatingWebhookConfiguration %s is ready", config.GetName())
			}
		}

		gomega.Expect(crds).ToNot(gomega.BeEmpty(), "at least one CRD should be found")
		gomega.Expect(crds).To(gomega.ContainElement(gomega.WithTransform(func(crd *apiextensionsv1.CustomResourceDefinition) bool {
			for _, version := range crd.Spec.Versions {
				if version.Subresources != nil {
					return version.Subresources.Status != nil || version.Subresources.Scale != nil
				}
			}
			return false
		}, gomega.BeTrue())), "at least one CRD should have status or scale subresource to approve it can be reconciled", format.Object(crds, 1))

		// check if the operator pods are running
		pods, err := f.ClientSet.CoreV1().Pods(operator.Namespace).List(ctx, metav1.ListOptions{})
		framework.ExpectNoError(err)
		gomega.Expect(pods.Items).ToNot(gomega.BeEmpty(), "at least one pod should be found in namespace %s", operator.Namespace)
		for _, pod := range pods.Items {
			err := e2epod.WaitForPodNameRunningInNamespace(ctx, f.ClientSet, pod.Name, operator.Namespace)
			framework.ExpectNoError(err)
		}
	})
})
