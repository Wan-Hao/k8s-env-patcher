#!/bin/bash

# Colors and emojis
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Help information
usage() {
    echo "Usage: $0 -c <cluster_name> -n <namespace> [-s] [-f]"
    echo "  -c: Cluster name"
    echo "  -n: Namespace"
    echo "  -s: Create new cluster (optional, default: use existing cluster)"
    echo "  -f: Force creation (optional, will delete existing resources if they exist)"
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

# Check if cluster exists
check_cluster_exists() {
    kind get clusters | grep -q "^$1$"
    return $?
}

# Check if namespace exists
check_namespace_exists() {
    kubectl get namespace "$1" >/dev/null 2>&1
    return $?
}

# Parameter processing
force_creation=false
while getopts "c:n:shf" opt; do
    case ${opt} in
        c )
            cluster_name=$OPTARG
            ;;
        n )
            namespace=$OPTARG
            ;;
        s )
            create_cluster=true
            ;;
        f )
            force_creation=true
            ;;
        h )
            usage
            ;;
        \? )
            usage
            ;;
    esac
done

# Check required parameters
if [ -z "$cluster_name" ] || [ -z "$namespace" ]; then
    error "Cluster name and namespace are required 🚫"
    usage
fi

info "Starting deployment process... 🚀"

# Cluster handling
if [ "$create_cluster" = true ]; then
    if check_cluster_exists "$cluster_name"; then
        if [ "$force_creation" = true ]; then
            info "Deleting existing cluster: $cluster_name 🗑️"
            kind delete cluster --name "$cluster_name"
        else
            error "Cluster $cluster_name already exists. Use -f to force recreation ⚠️"
        fi
    fi
    
    info "Creating new cluster: $cluster_name 🔧"
    if ! kind create cluster --name "$cluster_name"; then
        error "Failed to create cluster $cluster_name ❌"
    fi
    success "Cluster $cluster_name created successfully 🎉"
else
    info "Checking if cluster $cluster_name exists..."
    if ! check_cluster_exists "$cluster_name"; then
        error "Cluster $cluster_name does not exist. Use -s to create a new cluster ❌"
    fi
    success "Using existing cluster: $cluster_name ✅"
fi

# Check cluster environment
info "Checking cluster environment... 🔍"
if ! kubectl cluster-info > /dev/null 2>&1; then
    error "Cannot connect to Kubernetes cluster 🔌"
fi

if ! kubectl api-versions | grep -q "admissionregistration.k8s.io/v1"; then
    error "Cluster does not support admissionregistration.k8s.io/v1 ⚠️"
fi
success "Cluster environment check passed ✅"

# Namespace handling
info "Checking namespace: $namespace 📁"
if check_namespace_exists "$namespace"; then
    if [ "$force_creation" = true ]; then
        info "Deleting existing namespace: $namespace 🗑️"
        kubectl delete namespace "$namespace" --timeout=60s
        # Wait for namespace to be fully deleted
        while check_namespace_exists "$namespace"; do
            info "Waiting for namespace to be deleted..."
            sleep 2
        done
    else
        error "Namespace $namespace already exists. Use -f to force recreation ⚠️"
    fi
fi

info "Creating namespace: $namespace 📁"
if ! kubectl create namespace "$namespace"; then
    error "Failed to create namespace $namespace ❌"
fi
success "Namespace $namespace created successfully ✅"

info "Adding label to namespace: $namespace 🏷️"
if ! kubectl label namespace "$namespace" wh/envInjector=enabled --overwrite; then
    error "Failed to add label to namespace $namespace ❌"
fi
success "Namespace label added successfully ✅"

# Cleanup function
cleanup() {
    local exit_code=$?
    info "Performing cleanup..."
    
    # Return to original directory
    cd "$initial_dir" 2>/dev/null
    
    if [ $exit_code -ne 0 ]; then
        # Only cleanup if something failed
        if [ "$create_cluster" = true ]; then
            warn "Deleting cluster due to deployment failure..."
            kind delete cluster --name "$cluster_name" 2>/dev/null
        else
            if [ -n "$namespace" ]; then
                warn "Deleting namespace due to deployment failure..."
                kubectl delete namespace "$namespace" --timeout=30s 2>/dev/null
            fi
        fi
    fi
    
    exit $exit_code
}

# Set up trap for cleanup
trap cleanup EXIT ERR

# Store initial directory
initial_dir=$(pwd)

# Build image
info "Building Docker image... 🏗️"
if ! cd image; then
    error "Cannot find image directory 📂"
fi

if ! docker build -t k8s-env-injector:dev .; then
    error "Docker build failed 🚫"
fi
success "Docker image built successfully 🎉"

# Load image to cluster
info "Loading image to cluster... 📦"
if ! kind load docker-image k8s-env-injector:dev --name "$cluster_name"; then
    error "Failed to load image to cluster ❌"
fi
success "Image loaded to cluster successfully ✨"

# Replace namespace in deployment files
info "Updating namespace in deployment files... 📝"
cd ../deployment || error "Cannot find deployment directory 📂"
for file in *.yaml; do
    if [ -f "$file" ]; then
        info "Processing $file..."
        if ! sed -i '' "s/namespace: admin/namespace: $namespace/g" "$file"; then
            error "Failed to update namespace in $file ⚠️"
        fi
    fi
done
success "Deployment files updated successfully ✅"

# Generate certificates and configuration
info "Generating certificates and configuration... 🔐"
if ! ./webhook-create-signed-cert.sh --service env-injector-webhook-svc --secret env-injector-webhook-certs --namespace "$namespace"; then
    error "Failed to generate certificates ❌"
fi

if ! cat mutatingwebhook.yaml | ./webhook-patch-ca-bundle.sh > mutatingwebhook-ca-bundle.yaml; then
    error "Failed to generate CA bundle ❌"
fi
success "Certificates and configuration generated successfully 🔒"

# Deploy resources
info "Deploying resources... 🚀"
for resource in configmap.yaml deployment.yaml service.yaml mutatingwebhook-ca-bundle.yaml; do
    info "Deploying $resource..."
    if ! kubectl create -f "$resource" -n "$namespace"; then
        error "Failed to deploy $resource ❌"
    fi
done
success "Resources deployed successfully 🎉"

# Check deployment status
info "Checking deployment status... 🔍"
echo "Waiting for pods to be ready..."
sleep 5  # Give some time for pods to start

# Wait for pods to be ready with timeout
timeout=60
while [ $timeout -gt 0 ]; do
    if kubectl get pods -n "$namespace" | grep "env-injector-webhook-deployment" | grep -q "Running"; then
        success "Pods are running successfully 🎯"
        break
    fi
    sleep 2
    timeout=$((timeout-2))
    info "Waiting for pods to be ready... (${timeout}s remaining)"
done

if [ $timeout -le 0 ]; then
    error "Timeout waiting for pods to be ready ⏰"
fi

kubectl get pods -n "$namespace"

success "Deployment completed successfully! 🎉"
echo -e "\n${GREEN}You can now use the webhook in namespace: $namespace${NC} 🚀" 