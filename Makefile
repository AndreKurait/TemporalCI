.PHONY: build test lint clean deploy bootstrap

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
