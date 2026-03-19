---
title: Pipeline Config (.temporalci.yaml)
description: Configure CI pipelines with .temporalci.yaml in your repository.
---

TemporalCI reads `.temporalci.yaml` from the root of your repository to define the CI pipeline.

## Format

```yaml
steps:
  - name: build
    image: golang:1.23
    command: go build ./...
    timeout: 5m

  - name: test
    image: golang:1.23
    command: gotestsum --junitfile /tmp/test-results/results.xml -- ./... -v
    timeout: 10m
    depends_on: [build]

  - name: lint
    image: golangci/golangci-lint:v1.62
    command: golangci-lint run ./...
    depends_on: [build]
```

## Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `steps` | list | Yes | — | Ordered list of CI steps |
| `steps[].name` | string | Yes | — | Step name (shown in GitHub status) |
| `steps[].image` | string | Yes | — | Docker image for the step container |
| `steps[].command` | string | Yes | — | Shell command to execute (`sh -c`) |
| `steps[].timeout` | string | No | `10m` | Maximum execution time |
| `steps[].depends_on` | list | No | — | Steps that must pass before this one runs |

## DAG Execution

Steps with `depends_on` form a directed acyclic graph. Independent steps run in parallel. If a dependency fails, downstream steps are skipped.

```yaml
steps:
  - name: build
    image: golang:1.26
    command: go build ./...

  - name: unit-test
    image: golang:1.26
    command: go test ./...
    depends_on: [build]

  - name: lint
    image: golangci/golangci-lint:latest
    command: golangci-lint run
    depends_on: [build]

  - name: integration-test
    image: golang:1.26
    command: go test -tags=integration ./...
    depends_on: [unit-test]
```

In this example, `unit-test` and `lint` run in parallel after `build`. `integration-test` waits for `unit-test`.

## Default Pipeline

If no `.temporalci.yaml` is found, TemporalCI uses:

```yaml
steps:
  - name: build
    image: golang:1.23
    command: go build ./...
  - name: test
    image: golang:1.23
    command: go test ./...
```

## JUnit XML

To get rich test reporting, output JUnit XML to `/tmp/test-results/*.xml`:

```yaml
steps:
  - name: test
    image: golang:1.23
    command: |
      go install gotest.tools/gotestsum@latest
      gotestsum --junitfile /tmp/test-results/results.xml -- ./... -v
```

TemporalCI parses the JUnit XML and includes:
- Per-test pass/fail annotations on the Check Run
- Test count summary in the PR comment
- Failure messages with file + line references

## Language Examples

### Go

```yaml
steps:
  - name: build
    image: golang:1.23
    command: go build ./...
  - name: test
    image: golang:1.23
    command: go test ./... -v -race
  - name: vet
    image: golang:1.23
    command: go vet ./...
```

### Node.js

```yaml
steps:
  - name: install
    image: node:20
    command: npm ci
  - name: test
    image: node:20
    command: npm test
    depends_on: [install]
  - name: build
    image: node:20
    command: npm run build
    depends_on: [install]
```

### Python

```yaml
steps:
  - name: install
    image: python:3.12
    command: pip install -r requirements.txt
  - name: test
    image: python:3.12
    command: python -m pytest --junitxml=/tmp/test-results/results.xml
    depends_on: [install]
  - name: lint
    image: python:3.12
    command: ruff check .
    depends_on: [install]
```
