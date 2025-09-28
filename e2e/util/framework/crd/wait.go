package crd

import (
	"context"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/kubernetes/test/e2e/framework"
)

// WaitForCrdEstablishedAndNamesAccepted waits for the CRD to have the Established and NamesAccepted conditions with True status.
func WaitForCrdEstablishedAndNamesAccepted(ctx context.Context, client clientset.Interface, crdName string) error {
	err := framework.Gomega().Eventually(ctx, framework.GetObject(client.ApiextensionsV1().CustomResourceDefinitions().Get, crdName, metav1.GetOptions{})).
		WithTimeout(framework.Poll).
		Should(gomega.HaveField("Status.Conditions", gstruct.MatchElements(
			func(condition any) string {
				return string(condition.(apiextensionsv1.CustomResourceDefinitionCondition).Type)
			},
			gstruct.IgnoreExtras,
			gstruct.Elements{
				"NamesAccepted": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Status": gomega.Equal(apiextensionsv1.ConditionTrue),
				}),
				"Established": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Status": gomega.Equal(apiextensionsv1.ConditionTrue),
				}),
			},
		)))
	return err
}
