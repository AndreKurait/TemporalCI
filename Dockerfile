FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /worker ./cmd/worker
RUN CGO_ENABLED=0 go build -o /webhook ./cmd/webhook

FROM golang:1.26-alpine
RUN apk add --no-cache git ca-certificates curl python3 py3-pip && \
    pip3 install --break-system-packages awscli && \
    curl -fsSL https://get.helm.sh/helm-v4.1.3-linux-amd64.tar.gz | tar xz -C /tmp && \
    mv /tmp/linux-amd64/helm /usr/local/bin/helm && rm -rf /tmp/linux-amd64 && \
    curl -fsSL -o /usr/local/bin/kubectl https://dl.k8s.io/release/v1.35.3/bin/linux/amd64/kubectl && \
    chmod +x /usr/local/bin/kubectl
COPY --from=builder /worker /usr/local/bin/worker
COPY --from=builder /webhook /usr/local/bin/webhook
