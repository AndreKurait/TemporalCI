FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /worker ./cmd/worker
RUN CGO_ENABLED=0 go build -o /webhook ./cmd/webhook

FROM alpine:3.19
RUN apk add --no-cache git ca-certificates
COPY --from=builder /worker /usr/local/bin/worker
COPY --from=builder /webhook /usr/local/bin/webhook
