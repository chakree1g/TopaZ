#!/usr/bin/env bash
# hack/minikube-deploy.sh — Deploy TOPAS to a local Minikube cluster using Podman.
#
# Usage:
#   ./hack/minikube-deploy.sh          # full deploy (start minikube + build + deploy)
#   ./hack/minikube-deploy.sh teardown  # delete the minikube cluster
set -euo pipefail

IMG="${IMG:-localhost/topas-controller:latest}"
MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-minikube}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

red()   { printf '\033[0;31m%s\033[0m\n' "$*"; }
green() { printf '\033[0;32m%s\033[0m\n' "$*"; }
info()  { printf '\033[0;36m→ %s\033[0m\n' "$*"; }

# ─── Teardown ──────────────────────────────────────────────────────────
if [[ "${1:-}" == "teardown" ]]; then
    info "Deleting Minikube cluster '${MINIKUBE_PROFILE}'..."
    minikube delete -p "$MINIKUBE_PROFILE"
    green "Cluster deleted."
    exit 0
fi

# ─── Prerequisites ─────────────────────────────────────────────────────
for cmd in podman minikube kubectl go; do
    command -v "$cmd" >/dev/null 2>&1 || { red "Missing prerequisite: $cmd"; exit 1; }
done

# ─── 1. Start Minikube ─────────────────────────────────────────────────
if minikube status -p "$MINIKUBE_PROFILE" >/dev/null 2>&1; then
    info "Minikube '${MINIKUBE_PROFILE}' already running."
else
    info "Starting Minikube with Podman driver..."
    minikube start -p "$MINIKUBE_PROFILE" --driver=podman --container-runtime=containerd
fi

# ─── 2. Ensure Podman machine is running ───────────────────────────────
if ! podman machine inspect >/dev/null 2>&1; then
    info "Initializing Podman machine..."
    podman machine init || true
    podman machine start
fi

# ─── 3. Build controller image ─────────────────────────────────────────
info "Building binary locally..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/manager ./cmd/
info "Building controller image: ${IMG}"
podman build --no-cache -t "$IMG" "$ROOT"

# ─── 4. Load image into Minikube ───────────────────────────────────────
info "Loading image into Minikube..."
podman save "$IMG" | minikube image load --daemon=false -

# ─── 5. Install CRDs ──────────────────────────────────────────────────
info "Installing CRDs..."
make -C "$ROOT" install

# ─── 6. Deploy controller ─────────────────────────────────────────────
info "Deploying controller..."
make -C "$ROOT" deploy CONTAINER_TOOL=podman IMG="$IMG"

# ─── 7. Patch imagePullPolicy ─────────────────────────────────────────
info "Setting imagePullPolicy=Never for local image..."
kubectl patch deployment topas-controller-manager -n topas-system \
    --type=json \
    -p='[{"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"Never"}]'

# ─── 8. Wait for rollout ──────────────────────────────────────────────
info "Waiting for controller rollout..."
kubectl rollout status deployment/topas-controller-manager -n topas-system --timeout=90s

# ─── 9. Verify ─────────────────────────────────────────────────────────
echo ""
green "✅ TOPAS deployed successfully!"
echo ""
info "Controller pod:"
kubectl get pods -n topas-system
echo ""
info "Installed CRDs:"
kubectl get crds | grep apps.example.com
echo ""
info "Next steps:"
echo "  kubectl apply -f manifests/sample-app.yaml   # deploy a sample App"
echo "  kubectl apply -f manifests/test-run.yaml      # run a sample test"
echo "  kubectl logs -f -n topas-system deployment/topas-controller-manager  # watch logs"
echo "  ./hack/minikube-deploy.sh teardown            # cleanup"
