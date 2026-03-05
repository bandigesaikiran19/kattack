FROM golang:1.24-alpine AS builder

WORKDIR /workspace
RUN apk add --no-cache git curl ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager ./cmd/main.go

ARG VEGETA_VERSION=v12.13.0
RUN curl -sSL -o /tmp/vegeta.tar.gz https://github.com/tsenart/vegeta/releases/download/${VEGETA_VERSION}/vegeta_${VEGETA_VERSION#v}_linux_amd64.tar.gz \
    && tar -xzf /tmp/vegeta.tar.gz -C /tmp \
    && mv /tmp/vegeta /workspace/vegeta \
    && chmod +x /workspace/vegeta

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager /manager
COPY --from=builder /workspace/vegeta /usr/local/bin/vegeta
USER 65532:65532

ENTRYPOINT ["/manager"]
