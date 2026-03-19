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
minikube addons enable registry 2>/dev/null || true

# Build and push image to minikube registry
echo "Building and pushing image..."
cd "$ROOT_DIR"
docker build -t localhost:5000/temporalci:local .
minikube image load localhost:5000/temporalci:local
minikube ssh "docker push localhost:5000/temporalci:local"

# Helm dependency update
echo "Updating Helm dependencies..."
helm dependency build deploy/helm 2>/dev/null || true

# Install
echo "Installing TemporalCI..."
helm upgrade --install temporalci deploy/helm \
  -n temporalci --create-namespace \
  -f deploy/helm/values-local.yaml

echo ""
echo "=== Waiting for pods ==="
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=temporalci \
  -n temporalci --timeout=180s 2>/dev/null || true

echo ""
echo "=== TemporalCI Local Ready ==="
echo ""
echo "Port-forward (run in separate terminal):"
echo "  kubectl port-forward svc/temporalci-temporal-web -n temporalci 8088:8088"
echo "  kubectl port-forward svc/temporalci-webhook -n temporalci 8080:8080"
echo ""
echo "Trigger a test workflow:"
echo "  ./scripts/local-trigger.sh AndreKurait/TemporalCI-test"
