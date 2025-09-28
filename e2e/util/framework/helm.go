package framework

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	uexec "k8s.io/utils/exec"

	"k8s.io/kubernetes/test/e2e/framework"
)

// HelmBuilder is used to build, customize and execute a helm Command.
// Add more functions to customize the builder as needed.
type HelmBuilder struct {
	cmd *exec.Cmd
	// appendEnv contains only AppendEnv(...) values and NOT os.Environ()
	// logging os.Environ is ~redundant and noisy
	// logging the env we modified is important to understand what we're running
	// versus the defaults
	appendedEnv []string
	timeout     <-chan time.Time
}

// NewHelmCommand returns a HelmBuilder for running helm.
func NewHelmCommand(namespace string, args ...string) *HelmBuilder {
	b := new(HelmBuilder)

	defaultArgs := []string{}

	// Reference a --kube-apiserver option so tests can run anywhere.
	if framework.TestContext.Host != "" {
		defaultArgs = append(defaultArgs, "--kube-apiserver="+framework.TestContext.Host)
	}
	if framework.TestContext.KubeConfig != "" {
		defaultArgs = append(defaultArgs, "--kubeconfig="+framework.TestContext.KubeConfig)

		// Reference the KubeContext
		if framework.TestContext.KubeContext != "" {
			defaultArgs = append(defaultArgs, "--kube-context="+framework.TestContext.KubeContext)
		}
	}

	if namespace != "" {
		defaultArgs = append(defaultArgs, fmt.Sprintf("--namespace=%s", namespace))
	}
	helmArgs := append(defaultArgs, args...)

	b.cmd = exec.Command("helm", helmArgs...)
	return b
}

// AppendEnv appends the given environment and returns itself.
func (b *HelmBuilder) AppendEnv(env []string) *HelmBuilder {
	if b.cmd.Env == nil {
		b.cmd.Env = os.Environ()
	}
	b.cmd.Env = append(b.cmd.Env, env...)
	b.appendedEnv = append(b.appendedEnv, env...)
	return b
}

// WithTimeout sets the given timeout and returns itself.
func (b *HelmBuilder) WithTimeout(t <-chan time.Time) *HelmBuilder {
	b.timeout = t
	return b
}

// WithStdinData sets the given data to stdin and returns itself.
func (b HelmBuilder) WithStdinData(data string) *HelmBuilder {
	b.cmd.Stdin = strings.NewReader(data)
	return &b
}

// WithStdinReader sets the given reader and returns itself.
func (b HelmBuilder) WithStdinReader(reader io.Reader) *HelmBuilder {
	b.cmd.Stdin = reader
	return &b
}

// ExecOrDie runs the helm executable or dies if error occurs.
func (b HelmBuilder) ExecOrDie(namespace string) string {
	str, err := b.Exec()
	// In case of i/o timeout error, try talking to the apiserver again after 2s before dying.
	// Note that we're still dying after retrying so that we can get visibility to triage it further.
	if isTimeout(err) {
		framework.Logf("Hit i/o timeout error, talking to the server 2s later to see if it's temporary.")
		time.Sleep(2 * time.Second)
		retryStr, retryErr := RunHelm(namespace, "version")
		framework.Logf("stdout: %q", retryStr)
		framework.Logf("err: %v", retryErr)
	}
	framework.ExpectNoError(err)
	return str
}

func isTimeout(err error) bool {
	switch err := err.(type) {
	case *url.Error:
		if err, ok := err.Err.(net.Error); ok && err.Timeout() {
			return true
		}
	case net.Error:
		if err.Timeout() {
			return true
		}
	}
	return false
}

// Exec runs the helm executable.
func (b HelmBuilder) Exec() (string, error) {
	stdout, _, err := b.ExecWithFullOutput()
	return stdout, err
}

// ExecWithFullOutput runs the helm executable, and returns the stdout and stderr.
func (b HelmBuilder) ExecWithFullOutput() (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := b.cmd
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if len(b.appendedEnv) > 0 {
		framework.Logf("Running '%s %s %s'", strings.Join(b.appendedEnv, " "), cmd.Path, strings.Join(cmd.Args[1:], " ")) // skip arg[0] as it is printed separately
	} else {
		framework.Logf("Running '%s %s'", cmd.Path, strings.Join(cmd.Args[1:], " ")) // skip arg[0] as it is printed separately
	}
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("error starting %v:\nCommand stdout:\n%v\nstderr:\n%v\nerror:\n%v", cmd, cmd.Stdout, cmd.Stderr, err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Wait()
	}()
	select {
	case err := <-errCh:
		if err != nil {
			var rc = 127
			if ee, ok := err.(*exec.ExitError); ok {
				rc = int(ee.Sys().(syscall.WaitStatus).ExitStatus())
				framework.Logf("rc: %d", rc)
			}
			return stdout.String(), stderr.String(), uexec.CodeExitError{
				Err:  fmt.Errorf("error running %v:\nCommand stdout:\n%v\nstderr:\n%v\nerror:\n%v", cmd, cmd.Stdout, cmd.Stderr, err),
				Code: rc,
			}
		}
	case <-b.timeout:
		b.cmd.Process.Kill()
		return "", "", fmt.Errorf("timed out waiting for command %v:\nCommand stdout:\n%v\nstderr:\n%v", cmd, cmd.Stdout, cmd.Stderr)
	}
	framework.Logf("stderr: %q", stderr.String())
	framework.Logf("stdout: %q", stdout.String())
	return stdout.String(), stderr.String(), nil
}

// RunHelmOrDie is a convenience wrapper over helmBuilder
func RunHelmOrDie(namespace string, args ...string) string {
	return NewHelmCommand(namespace, args...).ExecOrDie(namespace)
}

// RunHelm is a convenience wrapper over helmBuilder
func RunHelm(namespace string, args ...string) (string, error) {
	return NewHelmCommand(namespace, args...).Exec()
}

// RunHelmWithFullOutput is a convenience wrapper over helmBuilder
// It will also return the command's stderr.
func RunHelmWithFullOutput(namespace string, args ...string) (string, string, error) {
	return NewHelmCommand(namespace, args...).ExecWithFullOutput()
}

// RunHelmOrDieInput is a convenience wrapper over helmBuilder that takes input to stdin
func RunHelmOrDieInput(namespace string, data string, args ...string) string {
	return NewHelmCommand(namespace, args...).WithStdinData(data).ExecOrDie(namespace)
}

// RunHelmInput is a convenience wrapper over helmBuilder that takes input to stdin
func RunHelmInput(namespace string, data string, args ...string) (string, error) {
	return NewHelmCommand(namespace, args...).WithStdinData(data).Exec()
}
