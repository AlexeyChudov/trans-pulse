# Сборка и запуск REST API: docker compose -f docker-compose.app.yml up -d --build
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
ENV CGO_ENABLED=0
# goose нужен для миграций (и для сервиса tests)
RUN GOBIN=/usr/local/bin go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
COPY . .
RUN go build -o /out/api ./cmd/api && go build -o /out/createadmin ./cmd/createadmin

FROM alpine:3.20 AS app
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /out/api /out/createadmin /usr/local/bin/
COPY --from=builder /usr/local/bin/goose /usr/local/bin/goose
COPY migrations /migrations
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
