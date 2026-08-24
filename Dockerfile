# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.2

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG MAIN_PACKAGE=./cmd

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/check-stateless-server \
    ${MAIN_PACKAGE}


FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && \
    adduser -S -G app app

WORKDIR /app

RUN mkdir -p /app/config /app/secrets && \
    chown -R app:app /app

COPY --from=builder /out/check-stateless-server /app/check-stateless-server

USER app

EXPOSE 8080

ENTRYPOINT ["/app/check-stateless-server"]
CMD ["-config", "/app/config/config.docker.yaml"]