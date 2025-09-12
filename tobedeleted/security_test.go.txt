package pkg

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// How we might test it: Deploy a Pod to a node with available accelerators, without requesting
// accelerator resources in the Pod spec. Execute a command in the Pod to probe for accelerator
// devices, and the command should fail or report that no accelerator devices are found. Create
// two Pods, each is allocated an accelerator resource. Execute a command in one Pod to attempt
// to access the other Pod’s accelerator, and should be denied.
//
// See https://docs.google.com/document/d/1hXoSdh9FEs13Yde8DivCYjjXyxa7j4J8erjZPEGWuzc/edit?tab=t.0
func TestSecureAcceleratorAccess(t *testing.T) {
	description := "Ensure that access to accelerators from within containers is properly isolated and mediated by " +
		"the Kubernetes resource management framework (device plugin or DRA) and container runtime, preventing unauthorized access or interference between workloads."

	f := features.New("secure_accelerator_access").
		WithLabel("type", "security").
		WithLabel("id", "secure_accelerator_access").
		WithLabel("level", "MUST").
		AssessWithDescription("should fail to probe for accelerator devices when the pod is not allocated an accelerator resource", description, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			targetPod := &corev1.Pod{}
			client := cfg.Client()
			createPodFn := func(ctx context.Context, obj k8s.Object) error {
				t.Logf("Creating pod with generate name %s", obj.GetGenerateName())
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return fmt.Errorf("expected a pod, got %T", obj)
				}

				t.Logf("Setting pod's namespace to %s", cfg.Namespace())
				pod.SetNamespace(cfg.Namespace())
				t.Logf("Remove accelerators resources from the requrests and limits of the pod's containers")
				podutils.VisitContainers(&pod.Spec, podutils.AllContainers, func(container *corev1.Container, containerType podutils.ContainerType) bool {
					for rName := range container.Resources.Limits {
						if rName == corev1.ResourceCPU || rName == corev1.ResourceMemory {
							continue
						}
						t.Logf("Removing %s from the container's resources limits", rName)
						delete(container.Resources.Limits, rName)
					}
					return true
				})
				err := client.Resources().Create(ctx, obj)
				if err != nil {
					return err
				}
				targetPod = pod
				t.Logf("Pod %s created", pod.GetName())
				return nil
			}
			err := decoder.DecodeEachFile(ctx, fs.FS(os.DirFS("testdata")), "secure_accelerator_access/pod-template.yaml", createPodFn, decoder.MutateNamespace(cfg.Namespace()))
			if err != nil {
				t.Error(err)
				return ctx
			}
			defer func() {
				t.Logf("Deleting pod %s", targetPod.GetName())
				client.Resources().Delete(ctx, targetPod)
			}()

			t.Logf("Waiting for pod %s to be ready", targetPod.GetName())
			err = wait.For(conditions.New(client.Resources()).PodReady(targetPod))
			if err != nil {
				t.Errorf("error when waiting for pod %s to be running: %v", targetPod.GetName(), err)
				return ctx
			}

			cmd := []string{"nvidia-smi"}
			t.Logf("Executing command %q in pod %s", cmd, targetPod.GetName())
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			err = client.Resources().ExecInPod(ctx, cfg.Namespace(), targetPod.GetName(), targetPod.Spec.Containers[0].Name, cmd, stdout, stderr)
			if err != nil || stderr.Len() > 0 {
				return ctx
			}
			t.Errorf("unexpected successful result, stdout: %s", stdout.String())
			return ctx
		}).
		AssessWithDescription("should be denied to access the accelerator devices of other pods", description, func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client := cfg.Client()

			// FIXME: nvidia-smi --query-gpu=index,name --format=csv,noheader
			// It is not supported by fake-gpu-operator.
			cmd := []string{"nvidia-smi"}

			pods := []*corev1.Pod{}
			createPodFn := func(ctx context.Context, obj k8s.Object) error {
				t.Logf("Creating pod with generate name %s", obj.GetGenerateName())
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return fmt.Errorf("expected a pod, got %T", obj)
				}
				err := client.Resources().Create(ctx, obj)
				if err != nil {
					return err
				}
				pods = append(pods, pod)
				return nil
			}

			t.Logf("Creating two pods")
			buffers := []*bytes.Buffer{}
			for i := range 2 {
				err := decoder.DecodeEachFile(ctx, fs.FS(os.DirFS("testdata")), "secure_accelerator_access/pod-template.yaml", createPodFn, decoder.MutateNamespace(cfg.Namespace()))
				if err != nil {
					t.Error(err)
					return ctx
				}

				t.Logf("Waiting for pod %s to be ready", pods[i].GetName())
				err = wait.For(conditions.New(client.Resources()).PodReady(pods[i]))
				if err != nil {
					t.Errorf("error when waiting for pod %s to be running: %v", pods[i].GetName(), err)
					return ctx
				}
				stdout := &bytes.Buffer{}
				stderr := &bytes.Buffer{}
				buffers = append(buffers, stdout)
				t.Logf("Executing command %q in pod %s", cmd, pods[i].GetName())
				err = client.Resources().ExecInPod(ctx, cfg.Namespace(), pods[i].GetName(), pods[i].Spec.Containers[0].Name, cmd, stdout, stderr)
				if err != nil || stderr.Len() > 0 {
					t.Errorf("error when executing command in pod %s: %v", pods[i].GetName(), err)
					return ctx
				}
				t.Logf("Got output from pod %s:\n %s", pods[i].GetName(), stdout.String())
			}

			if strings.EqualFold(buffers[0].String(), buffers[1].String()) {
				t.Error("the output from the two pods is the same which means they can access each other's accelerator devices")
				return ctx
			}
			return ctx
		})

	testenv.Test(t, f.Feature())
}
