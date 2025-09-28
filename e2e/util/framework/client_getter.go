package framework

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	"k8s.io/kubernetes/test/e2e/framework"
)

type clientGetter struct {
	restConfig     *rest.Config
	discoveryCache discovery.CachedDiscoveryInterface
	restMapper     meta.RESTMapper
}

var _ resource.RESTClientGetter = &clientGetter{}

func NewClientGetter(f *framework.Framework) *clientGetter {
	discoveryCache := memory.NewMemCacheClient(f.ClientSet.Discovery())
	return &clientGetter{
		restConfig:     f.ClientConfig(),
		discoveryCache: discoveryCache,
		restMapper:     restmapper.NewDeferredDiscoveryRESTMapper(discoveryCache),
	}
}

func (c *clientGetter) ToRESTConfig() (*rest.Config, error) {
	return c.restConfig, nil
}

func (c *clientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return c.discoveryCache, nil
}

func (c *clientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return c.restMapper, nil
}
