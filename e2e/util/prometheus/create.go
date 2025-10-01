package prometheus

import (
	"context"
	"encoding/json"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"

	"k8s.io/kubernetes/test/e2e/framework"
)

// CreateServiceMonitor creates a ServiceMonitor with the given namespace, name, matchLabels and port. If
// the namespace selector is not nil, the namespace is patched with the servicemonitor namespace selector
// of the prometheus instance. If the namespace selector is nil, the monitor namespace is set to the namespace
// of the prometheus instance.
func CreateServiceMonitor(ctx context.Context, promOpClient monitoring.Interface, prom monitoringv1.Prometheus, client clientset.Interface, namespace, name string, matchLabels map[string]string, port string) *monitoringv1.ServiceMonitor {
	labels, err := metav1.LabelSelectorAsMap(prom.Spec.ServiceMonitorSelector)
	framework.ExpectNoError(err, "error when converting label selector to map")

	smNamespace := namespace
	smNamespaceSelector := prom.Spec.ServiceMonitorNamespaceSelector
	if smNamespaceSelector != nil {
		nsLabels, err := metav1.LabelSelectorAsMap(smNamespaceSelector)
		framework.ExpectNoError(err, "error when converting label selector to map")

		if len(nsLabels) > 0 {
			nsPatch, err := json.Marshal(map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": nsLabels,
				},
			})
			framework.ExpectNoError(err, "error marshaling namespace patch")
			_, err = client.CoreV1().Namespaces().Patch(ctx, namespace, types.StrategicMergePatchType, nsPatch, metav1.PatchOptions{})
			framework.ExpectNoError(err, "error patching namespace")
		}
	} else {
		smNamespace = prom.Namespace
	}

	sm := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{namespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:     port,
					Interval: "15s",
					Path:     "/metrics",
				},
			},
		},
	}

	sm, err = promOpClient.MonitoringV1().ServiceMonitors(smNamespace).Create(ctx, sm, metav1.CreateOptions{})
	framework.ExpectNoError(err, "error when creating service monitor")
	return sm
}
