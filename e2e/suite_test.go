package e2e

import (
	"context"
	"flag"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
	"github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"
	frameworkconfig "k8s.io/kubernetes/test/e2e/framework/config"
	e2ereporters "k8s.io/kubernetes/test/e2e/reporters"
)

const kubeconfigEnvVar = "KUBECONFIG"

func init() {
	testing.Init()

	// k8s.io/kubernetes/test/e2e/framework requires env KUBECONFIG to be set
	// it does not fall back to defaults
	if os.Getenv(kubeconfigEnvVar) == "" {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		if _, err := os.Stat(kubeconfig); err == nil {
			_ = os.Setenv(kubeconfigEnvVar, kubeconfig)
		}
	}
	frameworkconfig.CopyFlags(frameworkconfig.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
	flag.Parse()

	framework.AfterReadingAllFlags(&framework.TestContext)
}

var progressReporter = &e2ereporters.ProgressReporter{}

var _ = ginkgo.SynchronizedBeforeSuite(func(ctx context.Context) []byte {
	progressReporter.SetStartMsg()
	return nil
}, func(ctx context.Context, data []byte) {})

var _ = ginkgo.SynchronizedAfterSuite(func() {
	progressReporter.SetEndMsg()
}, func(ctx context.Context) {})

var _ = ginkgo.ReportAfterEach(func(report ginkgo.SpecReport) {
	progressReporter.ProcessSpecReport(report)
})

var _ = ginkgo.ReportBeforeSuite(func(report ginkgo.Report) {
	progressReporter.SetTestsTotal(report.PreRunStats.SpecsThatWillRun)
})

func TestE2E(t *testing.T) {
	progressReporter = e2ereporters.NewProgressReporter(framework.TestContext.ProgressReportURL)
	gomega.RegisterFailHandler(framework.Fail)
	reportDir := framework.TestContext.ReportDir
	if reportDir == "" {
		reportDir = os.Getenv("ARTIFACTS")
	}
	t.Logf("ReportDir: %s", reportDir)
	if reportDir != "" {
		if err := os.MkdirAll(reportDir, 0755); err != nil {
			t.Fatalf("Failed creating report directory: %v", err)
		}

		ginkgo.ReportAfterSuite("Kubernetes AI Conformance e2e JUnit report", func(report ginkgo.Report) {
			// With Ginkgo v1, we used to write one file per
			// parallel node. Now Ginkgo v2 automatically merges
			// all results into a report for us. The 01 suffix is
			// kept in case that users expect files to be called
			// "junit_<prefix><number>.xml".
			junitReport := path.Join(reportDir, "junit_"+framework.TestContext.ReportPrefix+"01.xml")

			// writeJUnitReport generates a JUnit file in the e2e
			// report directory that is shorter than the one
			// normally written by `ginkgo --junit-report`. This is
			// needed because the full report can become too large
			// for tools like Spyglass
			// (https://github.com/kubernetes/kubernetes/issues/111510).
			framework.ExpectNoError(WriteJUnitReport(report, junitReport))
		})
	}
	suiteConfig, reporterConfig := framework.CreateGinkgoConfig()
	ginkgo.RunSpecs(t, "Kubernetes AI Conformance End-to-End Tests", suiteConfig, reporterConfig)
}

// WriteJUnitReport generates a JUnit file that is shorter than the one
// normally written by `ginkgo --junit-report`. This is needed because the full
// report can become too large for tools like Spyglass
// (https://github.com/kubernetes/kubernetes/issues/111510).
func WriteJUnitReport(report ginkgo.Report, filename string) error {
	config := reporters.JunitReportConfig{
		// Remove details for specs where we don't care.
		OmitTimelinesForSpecState: types.SpecStatePassed | types.SpecStateSkipped,

		// Don't write <failure message="summary">. The same text is
		// also in the full text for the failure. If we were to write
		// both, then tools like kettle and spyglass would concatenate
		// the two strings and thus show duplicated information.
		OmitFailureMessageAttr: true,
	}

	return reporters.GenerateJUnitReportWithConfig(report, filename, config)
}
