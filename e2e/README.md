> Lifted from https://github.com/kubernetes/kubernetes/tree/v1.34.0/test/e2e
> 
> Changed Items:
> - Import "_ github.com/carlory/ai-conformance/e2e/ai" to e2e_test.go as AI test sources.
> 
> - When setting `--clean-start` flag to true, only delete namespaces with label `e2e-framework` instead of all non-system namespaces.
>   It makes sure that we don't unintentionally delete namespaces which are created by vendors for their own components.
> 
> - Add a new wrapper function `frameworkutil.WGDescribe` to add a label with the pattern `wg-<name>` to the test. It's currently only
>   used in AI conformance tests in `github.com/carlory/ai-conformance/e2e/ai` package. It makes it easier to specify the AI tests 
>   via setting the `FOCUS` environment variable with the `\[wg-ai-conformance\]` label and the runner will run all levels of tests. 
>   The level concept comes from https://github.com/cncf/ai-conformance/blob/main/self-cert-questionnaire-template.yaml. It introduces
>   two levels for the AI tests: `MUST` and `SHOULD`. I map the `MUST` level to these ginkgo labels: `[AIConformance]` for these tests 
>   written in `github.com/carlory/ai-conformance/e2e/ai` package and `[Conformance]` for these tests written in the kubernetes/kuberentes
>   repository.
> 
> - Add a new wrapper function `frameworkutil.AIConformanceIt` to add `[Conformance]` and `[AIConformance]` labels. It's only used in 
>   AI conformance tests in `github.com/carlory/ai-conformance/e2e/ai` package. It make it possible for `hydrophone` and `sonobuoy`
>   to run the AI conformance tests and kubernetes conformance tests together when a custom image is used. And it also makes it easier
>   to specify the AI conformance tests via setting the `FOCUS` environment variable with the `[AIConformance]` label. 
> 
> - TODO: Another affected flag is that `--list-conformance-tests` flag will also show the AI conformance tests. (Not implemented yet)
>
> - Rename the test suite name from `Kubernetes e2e suite` to `Extended Kubernetes e2e suite with AI Conformance`.

# test/e2e

This is home to e2e tests used for presubmit, periodic, and postsubmit jobs.

Some of these jobs are merge-blocking, some are release-blocking.

## e2e test ownership

All e2e tests must adhere to the following policies:
- the test must be owned by one and only one SIG
- the test must live in/underneath a sig-owned package matching pattern: `test/e2e/[{subpath}/]{sig}/...`, e.g.
  - `test/e2e/auth` - all tests owned by sig-`auth`
  - `test/e2e/common/storage` - all tests `common` to cluster-level and node-level e2e tests, owned by sig-`node`
  - `test/e2e/upgrade/apps` - all tests used in `upgrade` testing, owned by sig-`apps`
- each sig-owned package should have an OWNERS file defining relevant approvers and labels for the owning sig, e.g.
```yaml
# test/e2e/node/OWNERS
# See the OWNERS docs at https://go.k8s.io/owners

approvers:
- alice
- bob
- cynthia
emeritus_approvers:
- dave
reviewers:
- sig-node-reviewers
labels:
- sig/node
```
- packages that use `{subpath}` should have an `imports.go` file importing sig-owned packages (for ginkgo's benefit), e.g.
```golang
// test/e2e/common/imports.go
package common

import (
	// ensure these packages are scanned by ginkgo for e2e tests
	_ "k8s.io/kubernetes/test/e2e/common/network"
	_ "k8s.io/kubernetes/test/e2e/common/node"
	_ "k8s.io/kubernetes/test/e2e/common/storage"
)
```
- test ownership must be declared via a top-level SIGDescribe call defined in the sig-owned package, e.g.
```golang
// test/e2e/lifecycle/framework.go
package lifecycle

import "k8s.io/kubernetes/test/e2e/framework"

// SIGDescribe annotates the test with the SIG label.
var SIGDescribe = framework.SIGDescribe("cluster-lifecycle")
```
```golang
// test/e2e/lifecycle/bootstrap/bootstrap_signer.go

package bootstrap

import (
	"github.com/onsi/ginkgo"
	"k8s.io/kubernetes/test/e2e/lifecycle"
)
var _ = lifecycle.SIGDescribe("cluster", feature.BootstrapTokens, func() {
  /* ... */
  ginkgo.It("should sign the new added bootstrap tokens", func(ctx context.Context) {
    /* ... */
  })
  /* etc */
})
```

These polices are enforced:
- via the merge-blocking presubmit job `pull-kubernetes-verify`
- which ends up running `hack/verify-e2e-test-ownership.sh`
- which can also be run via `make verify WHAT=e2e-test-ownership`

## more info

See [kubernetes/community/.../e2e-tests.md](https://git.k8s.io/community/contributors/devel/sig-testing/e2e-tests.md)
