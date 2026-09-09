#!/bin/sh
# Applies each service's own schema. Per-project databases are migrated later,
# at runtime, by object-builder when a project is created.
set -e

base="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}"

for pair in auth_service:auth company_service:company object_builder:object-builder; do
    db=${pair%%:*}
    dir=${pair##*:}
    echo "migrating $db"
    migrate -path "/m/$dir" -database "$base/$db?sslmode=disable" up
done

echo "migrations done"
