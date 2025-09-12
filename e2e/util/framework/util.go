package framework

import (
	"context"
	"strings"
	"sync"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

// DeleteNamespaces deletes all namespaces that match the given delete and skip filters.
// Filter is by simple strings.Contains; first skip filter, then delete filter.
// Returns the list of deleted namespaces or an error.
func DeleteNamespaces(ctx context.Context, c clientset.Interface, deleteFilter, deleteLabelFilter, skipFilter []string) ([]string, error) {
	ginkgo.By("Deleting namespaces")
	nsList, err := c.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	framework.ExpectNoError(err, "Failed to get namespace list")
	var deleted []string
	var wg sync.WaitGroup
OUTER:
	for _, item := range nsList.Items {
		for _, pattern := range skipFilter {
			if strings.Contains(item.Name, pattern) {
				continue OUTER
			}
		}
		if deleteFilter != nil {
			var shouldDelete bool
			for _, pattern := range deleteFilter {
				if strings.Contains(item.Name, pattern) {
					shouldDelete = true
					break
				}
			}
			if !shouldDelete {
				continue OUTER
			}
		}
		if deleteLabelFilter != nil {
			var shouldDelete bool
			for _, pattern := range deleteLabelFilter {
				if _, ok := item.Labels[pattern]; ok {
					shouldDelete = true
					break
				}
			}
			if !shouldDelete {
				continue OUTER
			}
		}
		wg.Add(1)
		deleted = append(deleted, item.Name)
		go func(nsName string) {
			defer wg.Done()
			defer ginkgo.GinkgoRecover()
			gomega.Expect(c.CoreV1().Namespaces().Delete(ctx, nsName, metav1.DeleteOptions{})).To(gomega.Succeed())
			framework.Logf("namespace : %v api call to delete is complete ", nsName)
		}(item.Name)
	}
	wg.Wait()
	return deleted, nil
}
