package framework

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"k8s.io/kubernetes/test/e2e/framework"
)

var sigRE = regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)

// WGDescribe returns a wrapper function for ginkgo.Describe which injects
// the WG name as annotation. The parameter should be lowercase with
// no spaces and no wg- or WG- prefix.
func WGDescribe(wg string) func(...interface{}) bool {
	if !sigRE.MatchString(wg) || strings.HasPrefix(wg, "wg-") {
		framework.RecordBug(framework.NewBug(fmt.Sprintf("WG label must be lowercase, no spaces and no wg- prefix, got instead: %q", wg), 1))
	}
	return func(args ...interface{}) bool {
		args = append([]interface{}{framework.WithLabel("wg-" + wg)}, args...)
		return framework.Describe(args...)
	}
}

// AIConformanceIt is wrapper function for ginkgo It. Adds "[Conformance]" and "[AIConformance]" tags and makes static analysis easier.
func AIConformanceIt(args ...interface{}) bool {
	args = append(args, ginkgo.Offset(1), framework.WithConformance(), framework.WithLabel("AIConformance"))
	return framework.It(args...)
}
