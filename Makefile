.PHONY: build run test migrate-up migrate-down seed docker-up docker-down lint

build:
	cd backend && go build -o bin/api ./cmd/api

run: build
	cd backend && ./bin/api

test:
	cd backend && go test -v -count=1 ./tests/integration/...

migrate-up:
	migrate -path backend/migrations -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}" up

migrate-down:
	migrate -path backend/migrations -database "postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}" down

seed:
	PGPASSWORD="${DB_PASSWORD}" psql -h ${DB_HOST} -p ${DB_PORT} -U ${DB_USER} -d ${DB_NAME} -f backend/migrations/seed.sql

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v

lint:
	cd backend && golangci-lint run ./...
