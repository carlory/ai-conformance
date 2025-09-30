#!/usr/bin/env bash

PS4='+ [${BASH_SOURCE}:${LINENO}] '
set -x
set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SONOBUOY_PLUGIN_FILE=${SONOBUOY_PLUGIN_FILE:-""}
E2E_RESULTS_DIR=${E2E_RESULTS_DIR:-""}

function cleanup {
    if [ "$USE_EXISTING_CLUSTER" == 'false' ]
    then
        $KIND delete cluster --name "$KIND_CLUSTER_NAME"
    fi
}
function startup {
    if [ "$USE_EXISTING_CLUSTER" == 'false' ]
    then
        $KIND create cluster --name "$KIND_CLUSTER_NAME" --image "$E2E_KIND_NODE_VERSION" --config "$SCRIPT_DIR/kind-config.yaml"
        install_dependencies
    fi
}
function kind_load {
    $KIND load docker-image "$IMG" --name "$KIND_CLUSTER_NAME"
}
function install_dependencies {
    echo "Installing Gateway API CRDs"
    $KUBECTL apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.3.0/standard-install.yaml

    echo "Installing Prometheus stack"
    $HELM upgrade --install kube-prometheus-stack oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack \
        --namespace monitoring \
        --create-namespace \
        --wait \
        --timeout 15m

    echo "Installing Prometheus adapter with custom rules"
    $HELM upgrade --install prometheus-adapter oci://ghcr.io/prometheus-community/charts/prometheus-adapter \
        --namespace monitoring \
        --create-namespace \
        --values "$SCRIPT_DIR/helm/prometheus-adapter.yaml" \
        --wait \
        --timeout 15m

    echo "Installing Kueue"
    $HELM upgrade --install kueue oci://registry.k8s.io/kueue/charts/kueue \
        --version 0.13.4 \
        --namespace kueue-system \
        --create-namespace \
        --values "$SCRIPT_DIR/helm/kueue.yaml" \
        --wait \
        --timeout 15m

    echo "Installing fake GPU operator"
    $HELM upgrade --install gpu-operator oci://ghcr.io/run-ai/fake-gpu-operator/fake-gpu-operator \
        --version 0.0.63 \
        --namespace gpu-operator \
        --create-namespace \
        --wait \
        --timeout 15m

    echo "Creating ServiceMonitor for nvidia-dcgm-exporter"
    $KUBECTL apply -f - <<'EOF'
    apiVersion: monitoring.coreos.com/v1
    kind: ServiceMonitor
    metadata:
        name: nvidia-dcgm-exporter
        namespace: gpu-operator
        labels:
            release: kube-prometheus-stack
    spec:
        selector:
            matchLabels:
                app: nvidia-dcgm-exporter
        namespaceSelector:
            matchNames:
                - gpu-operator
        endpoints:
            - port: gpu-metrics
              interval: 30s
EOF
}
function run_e2e_tests {
    echo "Starting E2E tests"
    
    # Parse comma-separated E2E_EXTRA_ARGS into an array and convert to proper args
    local extra_args=()
    if [ -n "$E2E_EXTRA_ARGS" ]; then
        IFS=',' read -ra ARGS_ARRAY <<< "$E2E_EXTRA_ARGS"
        for arg in "${ARGS_ARRAY[@]}"; do
            # Trim whitespace and add to array
            extra_args+=("$(echo "$arg" | xargs)")
        done
    fi
    
    # Run ginkgo with parsed extra args
    "$GINKGO" -v --procs=1 --focus="$GINKGO_FOCUS" --skip="$GINKGO_SKIP" "$SCRIPT_DIR/../e2e" -- --kubeconfig "$HOME/.kube/config" "${extra_args[@]}"
    echo "Finished E2E tests"
}
function run_sonobuoy {
    if [ "$USE_EXISTING_CLUSTER" == 'false' ]
    then
        kind_load
    fi
    echo "Starting Sonobuoy..."
    $SONOBUOY run --plugin "$SONOBUOY_PLUGIN_FILE" --force-image-pull-policy --image-pull-policy IfNotPresent --kubeconfig "$HOME/.kube/config" --level debug --wait
    if [ "$E2E_RESULTS_DIR" != '' ]
    then
        echo "Show Sonobuoy status"
        $SONOBUOY status --kubeconfig "$HOME/.kube/config"
        echo "Retrieving Sonobuoy results..."
        mkdir -p "$E2E_RESULTS_DIR"
        $SONOBUOY retrieve --kubeconfig "$HOME/.kube/config" "$E2E_RESULTS_DIR"
    fi
}
function run_hydrophone {
    echo "Starting Hydrophone..."
    if [ "$USE_EXISTING_CLUSTER" == 'false' ]
    then
        kind_load
    fi
    mkdir -p "$E2E_RESULTS_DIR"
    
    $HYDROPHONE --conformance-image "$IMG" --focus="$GINKGO_FOCUS" --skip="$GINKGO_SKIP" --output-dir "$E2E_RESULTS_DIR" --extra-args="$E2E_EXTRA_ARGS"
}
function run {
    case "$E2E_TEST_RUNNER" in
        ""|"binary")
            run_e2e_tests
            ;;
        "sonobuoy")
            run_sonobuoy
            ;;
        "hydrophone")
            run_hydrophone
            ;;
        *)
            echo "❌ Invalid E2E test runner: $E2E_TEST_RUNNER"
            exit 1
            ;;
    esac
}
trap cleanup EXIT
startup
run
