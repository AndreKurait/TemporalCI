.PHONY: build test lint clean deploy redeploy

HELM_RELEASE ?= temporalci
HELM_NAMESPACE ?= temporalci
HELM_VALUES ?= deploy/helm/values-local.yaml

build:
	go build ./...

test:
	go test ./... -v

lint:
	go vet ./...

clean:
	rm -rf /tmp/ci

deploy:
	helm upgrade --install $(HELM_RELEASE) deploy/helm \
		-n $(HELM_NAMESPACE) --create-namespace \
		-f $(HELM_VALUES)

redeploy:
	helm upgrade $(HELM_RELEASE) deploy/helm \
		-n $(HELM_NAMESPACE) \
		-f $(HELM_VALUES) --force
