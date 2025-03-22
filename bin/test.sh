#!/bin/bash

# Colors and emojis
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Help information
usage() {
    echo "Usage: $0 [-n <namespace>]"
    echo "  -n: Namespace (optional, default: injector)"
    exit 1
}

# Status functions
info() {
    echo -e "${GREEN}[✨ INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[⚠️  WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[❌ ERROR]${NC} $1"
    exit 1
}

success() {
    echo -e "${GREEN}[✅ SUCCESS]${NC} $1"
}

# Parameter processing
namespace="injector"
while getopts "n:h" opt; do
    case ${opt} in
        n )
            namespace=$OPTARG
            ;;
        h )
            usage
            ;;
        \? )
            usage
            ;;
    esac
done

echo "namespace: $namespace"

# Check cluster context
info "Checking cluster context... 🔍"
if ! kubectl cluster-info > /dev/null 2>&1; then
    error "Cannot connect to Kubernetes cluster 🔌"
fi

# Check webhook deployment
info "Checking webhook deployment... 🔍"
if ! kubectl get pods -n "$namespace" | grep -q "env-injector-webhook-deployment.*Running"; then
    error "Webhook deployment is not running in namespace $namespace ❌"
fi
success "Webhook deployment is running ✅"

# Create test namespace if not exists
test_namespace="test-env-injector"
info "Setting up test namespace: $test_namespace 📁"
if ! kubectl get namespace "$test_namespace" > /dev/null 2>&1; then
    kubectl create namespace "$test_namespace"
fi

# Label the test namespace
info "Labeling test namespace... 🏷️"
kubectl label namespace "$test_namespace" wh/envInjector=enabled --overwrite

# Deploy test pods
info "Deploying test pods... 🚀"
test_files=("test_deployment.yaml" "test_deployment_no_labels.yaml" "test_deployment_wrong_type.yaml")
for file in "${test_files[@]}"; do
    info "Applying $file..."
    if ! kubectl apply -f "test/$file" -n "$test_namespace"; then
        error "Failed to apply $file ❌"
    fi
done

# Wait for pods to be ready
info "Waiting for pods to be ready... ⏳"
sleep 5

# Check pod status
timeout=60
while [ $timeout -gt 0 ]; do
    if kubectl get pods -n "$test_namespace" | grep -q "Running"; then
        break
    fi
    sleep 2
    timeout=$((timeout-2))
    info "Waiting for pods... (${timeout}s remaining)"
done

if [ $timeout -le 0 ]; then
    error "Timeout waiting for pods to be ready ⏰"
fi

# Check environment variables
info "Checking environment variables... 🔍"

# Check pod with correct labels
echo -e "\n=== Pod with correct labels ==="
if kubectl get pod -n "$test_namespace" -l app=sleep -o json | jq -e '.items[0].spec.containers[0].env[] | select(.name=="INJECTOR_TEST")' > /dev/null; then
    success "Environment variable correctly injected in pod with correct labels ✅"
else
    error "Environment variable not found in pod with correct labels ❌"
fi

# Check pod without labels
echo -e "\n=== Pod without labels ==="
if ! kubectl get pod -n "$test_namespace" -l app=sleep-no-labels -o json | jq -e '.items[0].spec.containers[0].env[] | select(.name=="INJECTOR_TEST")' > /dev/null 2>&1; then
    success "No environment variable injected in pod without labels (expected) ✅"
else
    error "Unexpected environment variable found in pod without labels ❌"
fi

# Check pod with wrong type
echo -e "\n=== Pod with wrong app-type ==="
if ! kubectl get pod -n "$test_namespace" -l app=sleep-wrong-type -o json | jq -e '.items[0].spec.containers[0].env[] | select(.name=="INJECTOR_TEST")' > /dev/null 2>&1; then
    success "No environment variable injected in pod with wrong type (expected) ✅"
else
    error "Unexpected environment variable found in pod with wrong type ❌"
fi

# Cleanup
info "Cleaning up test resources... 🧹"
kubectl delete namespace "$test_namespace"

success "All tests completed successfully! 🎉"
echo -e "\nTest Summary:"
echo -e "✅ Webhook deployment check passed"
echo -e "✅ Pod with correct labels: environment variable injected"
echo -e "✅ Pod without labels: no environment variable injected"
echo -e "✅ Pod with wrong type: no environment variable injected" 