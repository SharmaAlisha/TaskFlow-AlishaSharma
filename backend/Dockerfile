## Stage 1: Build Go binary
FROM golang:1.24-alpine AS builder
ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /api ./cmd/api

## Stage 2: Get golang-migrate binary (pin compatible with Go toolchain in image)
FROM golang:1.24-alpine AS migrate-builder
ENV GOTOOLCHAIN=auto
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1

## Stage 3: Minimal runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl bash postgresql-client
COPY --from=builder /api /api
COPY --from=migrate-builder /go/bin/migrate /usr/local/bin/migrate
COPY --from=builder /app/migrations /migrations
COPY --from=builder /app/migrations/seed.sql /seed.sql
COPY --from=builder /app/scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/api"]
