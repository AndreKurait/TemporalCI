.PHONY: build test lint clean deploy bootstrap local-build local-up local-down local-restart local-logs-worker local-logs-webhook local-port-forward

build:
	go build ./...

test:
	go test ./... -v

lint:
	go vet ./...

clean:
	rm -rf /tmp/ci

# Bootstrap entire infrastructure + GitOps from scratch
bootstrap:
	./scripts/bootstrap.sh

# Manual deploy (for development; production uses ArgoCD)
deploy:
	helm upgrade --install temporalci deploy/helm \
		-n temporalci --create-namespace \
		-f deploy/helm/values.yaml \
		-f deploy/helm/values-eks.yaml

# --- Local development (minikube) ---

local-build:
	docker build -t localhost:5000/temporalci:local .
	minikube image load localhost:5000/temporalci:local
	minikube ssh "docker push localhost:5000/temporalci:local"

local-up: local-build
	helm dependency build deploy/helm 2>/dev/null || true
	helm upgrade --install temporalci deploy/helm \
		-n temporalci --create-namespace \
		-f deploy/helm/values-local.yaml

local-down:
	helm uninstall temporalci -n temporalci 2>/dev/null || true

local-restart: local-build
	kubectl rollout restart deployment/temporalci-ci-worker -n temporalci
	kubectl rollout restart deployment/temporalci-webhook -n temporalci

local-logs-worker:
	kubectl logs -f deployment/temporalci-ci-worker -n temporalci

local-logs-webhook:
	kubectl logs -f deployment/temporalci-webhook -n temporalci

local-port-forward:
	@echo "Temporal Web UI: http://localhost:8088"
	@echo "Webhook:         http://localhost:8080"
	kubectl port-forward svc/temporalci-temporal-web -n temporalci 8088:8088 &
	kubectl port-forward svc/temporalci-webhook -n temporalci 8080:8080 &

# --- CI Dashboard ---

dashboard-install:
	cd ui && npm ci

dashboard-build:
	cd ui && npm run build

dashboard-dev:
	cd ui && npm run dev

docker-build-dashboard:
	docker build -t temporalci-dashboard:latest ./ui

docker-build-all: local-build docker-build-dashboard
