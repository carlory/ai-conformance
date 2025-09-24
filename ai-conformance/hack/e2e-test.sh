#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SONOBUOY_PLUGIN_FILE=${SONOBUOY_PLUGIN_FILE:-""}
OUTPUT_DIR=${OUTPUT_DIR:-""}

function cleanup {
    if [ "$USE_EXISTING_CLUSTER" == 'false' ]
    then
        $KIND delete cluster --name "$KIND_CLUSTER_NAME"
    fi
    # TODO: revert all the changes made to this repo
}
function startup {
    if [ "$USE_EXISTING_CLUSTER" == 'false' ]
    then
        $KIND create cluster --name "$KIND_CLUSTER_NAME" --image "$E2E_KIND_NODE_VERSION" --config ./hack/kind-config.yaml
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
    cd "$SCRIPT_DIR/../pkg" && ./e2e.test -test.v -test.timeout=30m -kubeconfig "$HOME/.kube/config"
    echo "Finished E2E tests"
}
function run_sonobuoy {
    if [ "$USE_EXISTING_CLUSTER" == 'false' ]
    then
        kind_load
    fi
    echo "Starting Sonobuoy..."
    $SONOBUOY run --plugin "$SONOBUOY_PLUGIN_FILE" --force-image-pull-policy --image-pull-policy IfNotPresent --kubeconfig "$HOME/.kube/config" --level debug --wait
    if [ "$OUTPUT_DIR" != '' ]
    then
        echo "Retrieving Sonobuoy results..."
        mkdir -p "$OUTPUT_DIR"
        $SONOBUOY retrieve --kubeconfig "$HOME/.kube/config" "$OUTPUT_DIR"
        
        echo "Checking Sonobuoy status..."
        failures=$($SONOBUOY status --kubeconfig "$HOME/.kube/config" --json | $JQ -c '.plugins[].progress.failures // []')
        if [ "$failures" != "[]" ]; then
            echo "❌ Test failures detected: $failures"
            exit 1
        else
            echo "✅ All tests passed (no failures)"
        fi

    fi
}
function run_hydrophone {
    echo "Starting Hydrophone..."
    hydrophone
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
install_dependencies

# Wait for all the crds to be available.
sleep 5
run
