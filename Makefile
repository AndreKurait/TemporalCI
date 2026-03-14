.PHONY: build test lint clean

build:
	go build ./...

test:
	go test ./... -v

lint:
	go vet ./...

clean:
	rm -rf /tmp/ci
