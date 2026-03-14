FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /worker ./cmd/worker
RUN CGO_ENABLED=0 go build -o /webhook ./cmd/webhook

FROM golang:1.23-alpine
RUN apk add --no-cache git ca-certificates
COPY --from=builder /worker /usr/local/bin/worker
COPY --from=builder /webhook /usr/local/bin/webhook
