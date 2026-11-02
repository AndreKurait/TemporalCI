#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== TemporalCI Local Setup ==="

# Start minikube if not running
if ! minikube status &>/dev/null; then
  echo "Starting minikube..."
  minikube start --cpus=8 --memory=16384 --driver=docker
fi

# Enable registry addon
echo "Enabling minikube registry..."
minikube addons enable registry

# Keep local credentials out of values files and preserve them across upgrades.
kubectl --context minikube create namespace temporalci --dry-run=client -o yaml | kubectl --context minikube apply -f -
python3 "$SCRIPT_DIR/prepare-local-secrets.py"

# Build and push image to minikube registry
echo "Building and pushing image..."
cd "$ROOT_DIR"
docker build -t localhost:5000/temporalci:local .
minikube image load localhost:5000/temporalci:local
minikube ssh "docker push localhost:5000/temporalci:local"

# Helm dependency update
echo "Updating Helm dependencies..."
make helm-dependencies

# Install
echo "Installing TemporalCI..."
helm upgrade --install temporalci deploy/helm \
  --kube-context minikube \
  -n temporalci --create-namespace \
  -f deploy/helm/values-local.yaml

echo ""
echo "=== Waiting for pods ==="
kubectl --context minikube wait --for=condition=ready pod -l app.kubernetes.io/name=temporalci \
  -n temporalci --timeout=180s

echo ""
echo "=== TemporalCI Local Ready ==="
echo ""
echo "Port-forward (run in separate terminal):"
echo "  kubectl port-forward svc/temporalci-web -n temporalci 8088:8080"
echo "  kubectl port-forward svc/temporalci-webhook -n temporalci 8080:8080"
echo ""
echo "Trigger a test workflow:"
echo "  ./scripts/local-trigger.sh owner/repository"
