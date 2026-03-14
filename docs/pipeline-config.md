# Pipeline Configuration

TemporalCI reads `.temporalci.yaml` from the root of your repository to define the CI pipeline.

## Format

```yaml
steps:
  - name: build
    image: golang:1.23
    command: go build ./...
    timeout: 5m          # optional

  - name: test
    image: golang:1.23
    command: gotestsum --junitfile /tmp/test-results/results.xml -- ./... -v
    timeout: 10m

  - name: lint
    image: golangci/golangci-lint:v1.62
    command: golangci-lint run ./...
```

## Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `steps` | list | Yes | — | Ordered list of CI steps |
| `steps[].name` | string | Yes | — | Step name (shown in GitHub Check Run) |
| `steps[].image` | string | Yes | — | Docker image for the step container |
| `steps[].command` | string | Yes | — | Shell command to execute (`sh -c`) |
| `steps[].timeout` | string | No | `10m` | Maximum execution time |

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

To get rich test reporting in GitHub Check Runs, output JUnit XML to `/tmp/test-results/*.xml`:

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

## Examples

### Go Project

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

### Node.js Project

```yaml
steps:
  - name: install
    image: node:20
    command: npm ci
  - name: test
    image: node:20
    command: npm test -- --reporter=junit --outputFile=/tmp/test-results/results.xml
  - name: build
    image: node:20
    command: npm run build
```

### Python Project

```yaml
steps:
  - name: install
    image: python:3.12
    command: pip install -r requirements.txt
  - name: test
    image: python:3.12
    command: python -m pytest --junitxml=/tmp/test-results/results.xml
  - name: lint
    image: python:3.12
    command: ruff check .
```
