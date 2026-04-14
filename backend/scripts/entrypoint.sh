#!/bin/bash
set -e

DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}"

echo "Waiting for database at ${DB_HOST}:${DB_PORT}..."
until pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -q 2>/dev/null; do
    echo "  ...database not ready, retrying in 1s"
    sleep 1
done
echo "Database is ready."

echo "Running migrations..."
migrate -path /migrations -database "$DB_URL" up
echo "Migrations complete."

if [ "${SEED_DB}" = "true" ]; then
    echo "Seeding database..."
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f /seed.sql --set ON_ERROR_STOP=off 2>/dev/null || echo "Seed completed (some records may already exist)."
    echo "Seeding complete."
fi

echo "Starting API server..."
exec "$@"
