#!/usr/bin/env bash
set -euo pipefail

# TemporalCI Bootstrap Script
# Usage: ./scripts/bootstrap.sh [--github-token TOKEN] [--webhook-secret SECRET]
#
# Prerequisites:
#   - AWS CLI configured with appropriate credentials
#   - terraform, kubectl, helm, argocd CLI installed
#   - GitHub repo: AndreKurait/TemporalCI

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
CLUSTER_NAME="temporalci"
REGION="us-east-1"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
WEBHOOK_SECRET="${WEBHOOK_SECRET:-$(openssl rand -hex 20)}"

while [[ $# -gt 0 ]]; do
  case $1 in
    --github-token) GITHUB_TOKEN="$2"; shift 2 ;;
    --webhook-secret) WEBHOOK_SECRET="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

echo "=== TemporalCI Bootstrap ==="
echo "Cluster: $CLUSTER_NAME"
echo "Region:  $REGION"

# --- Step 1: Terraform ---
echo ""
echo "--- Step 1: Infrastructure (Terraform) ---"
cd "$ROOT_DIR/deploy/terraform"

# Create S3 backend bucket if it doesn't exist
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
BUCKET="temporalci-tfstate-${ACCOUNT_ID}"
if ! aws s3 head-bucket --bucket "$BUCKET" 2>/dev/null; then
  echo "Creating Terraform state bucket: $BUCKET"
  aws s3 mb "s3://$BUCKET" --region "$REGION"
  aws s3api put-bucket-versioning --bucket "$BUCKET" --versioning-configuration Status=Enabled
fi

terraform init -backend-config="bucket=$BUCKET"
terraform apply -auto-approve

# --- Step 2: Configure kubectl ---
echo ""
echo "--- Step 2: Configure kubectl ---"
aws eks update-kubeconfig --name "$CLUSTER_NAME" --region "$REGION"

echo "Waiting for nodes to be ready..."
for i in $(seq 1 30); do
  READY=$(kubectl get nodes --no-headers 2>/dev/null | grep -c " Ready" || true)
  [ "$READY" -ge 2 ] && break
  sleep 20
done

# --- Step 3: Create Temporal namespace ---
echo ""
echo "--- Step 3: Create Temporal namespace ---"
kubectl create namespace temporalci --dry-run=client -o yaml | kubectl apply -f -

# Create the secrets (ArgoCD will manage the rest)
kubectl create secret generic temporalci-secrets \
  --namespace temporalci \
  --from-literal=github-token="$GITHUB_TOKEN" \
  --from-literal=github-webhook-secret="$WEBHOOK_SECRET" \
  --dry-run=client -o yaml | kubectl apply -f -

# --- Step 4: Install ArgoCD ---
echo ""
echo "--- Step 4: Install ArgoCD ---"
kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

echo "Waiting for ArgoCD to be ready..."
kubectl wait --for=condition=available deployment/argocd-server -n argocd --timeout=300s

# --- Step 5: Apply ArgoCD Application ---
echo ""
echo "--- Step 5: Deploy TemporalCI via ArgoCD ---"
kubectl apply -f "$ROOT_DIR/deploy/argocd/application.yaml"

echo ""
echo "=== Bootstrap Complete ==="
echo ""
echo "ArgoCD will now sync the Helm chart from Git."
echo "Monitor: kubectl get application temporalci -n argocd"
echo ""
echo "To access ArgoCD UI:"
echo "  kubectl port-forward svc/argocd-server -n argocd 8443:443"
echo "  Password: kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d"
echo ""
echo "Webhook URL (after LB provisioning):"
echo "  kubectl get svc temporalci-webhook-lb -n temporalci -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'"
echo ""
if [ -n "$WEBHOOK_SECRET" ]; then
  echo "Webhook Secret: $WEBHOOK_SECRET"
fi
