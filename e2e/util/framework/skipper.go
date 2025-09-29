package framework

import (
	"context"
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

// SkipUnlessClusterAutoscalerExists skips the test if no supported cluster autoscaler has been installed.
func SkipUnlessClusterAutoscalerExists(ctx context.Context, client clientset.Interface) {
	autoscalers := map[string]func() bool{
		// Check if Cloud Autoscaler is enabled by trying to get its ConfigMap.
		"k8s.io/autoscaler/cluster-autoscaler": func() bool {
			_, err := client.CoreV1().ConfigMaps("kube-system").Get(ctx, "cluster-autoscaler-status", metav1.GetOptions{})
			return err == nil
		},
		// Check if Karpenter is enabled by trying to get its API resources.
		"sigs.k8s.io/karpenter": func() bool {
			_, err := client.Discovery().ServerResourcesForGroupVersion("karpenter.sh/v1")
			return err == nil
		},
	}

	for _, fn := range autoscalers {
		if fn() {
			return
		}
	}
	e2eskipper.Skipf("no cluster autoscaler has been installed: %v", maps.Keys(autoscalers))
}
