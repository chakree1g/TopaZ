---
description: Deploy TOPAS to a local Minikube cluster using Podman
---

# Deploy TOPAS to Minikube (Podman)

## Prerequisites
Ensure podman, minikube, go, and kubectl are installed:
```bash
brew install podman minikube
```

## Quick Deploy (Script)
// turbo
```bash
chmod +x hack/minikube-deploy.sh && ./hack/minikube-deploy.sh
```

## Manual Steps

### 1. Start Podman machine
```bash
podman machine init   # first time only
podman machine start
```

### 2. Start Minikube
```bash
minikube start --driver=podman --container-runtime=containerd
```

### 3. Build controller image
// turbo
```bash
podman build --no-cache -t localhost/topas-controller:latest .
```

### 4. Load image into Minikube
// turbo
```bash
podman save localhost/topas-controller:latest | minikube image load --daemon=false -
```

### 5. Install CRDs
// turbo
```bash
make install
```

### 6. Deploy controller
```bash
make deploy CONTAINER_TOOL=podman IMG=localhost/topas-controller:latest
```

### 7. Set imagePullPolicy to Never (local image)
```bash
kubectl patch deployment topas-controller-manager -n topas-system \
  --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"Never"}]'
```

### 8. Wait for rollout
// turbo
```bash
kubectl rollout status deployment/topas-controller-manager -n topas-system --timeout=90s
```

### 9. Verify
// turbo
```bash
kubectl get pods -n topas-system
kubectl get crds | grep apps.example.com
```

### 10. Apply sample manifests
```bash
kubectl apply -f manifests/sample-app.yaml
kubectl apply -f manifests/test-run.yaml
```

## Teardown
```bash
minikube delete
```
