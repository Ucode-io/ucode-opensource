package main

import (
	"fmt"
	"os"
)

// defaultEnv is the .env written on first run. It mirrors deploy/.env.example
// except for the signing key, which is generated per installation.
//
// UCODE_IMAGE_PREFIX and UCODE_VERSION are carried through from the process
// environment when set, so a checkout can run `make images && UCODE_VERSION=local
// ucode start` and exercise the images it just built instead of pulling.
func defaultEnv(secretKey string) string {
	images := ""
	if prefix := os.Getenv("UCODE_IMAGE_PREFIX"); prefix != "" {
		images += "UCODE_IMAGE_PREFIX=" + prefix + "\n"
	}
	if version := os.Getenv("UCODE_VERSION"); version != "" {
		images += "UCODE_VERSION=" + version + "\n"
	}
	if images != "" {
		images = "\n# Taken from the environment at first run.\n" + images
	}

	return images + fmt.Sprintf(`# Written by "ucode start" on first run. Safe to edit; it is not overwritten.
#
# Everything here is local-only. The stack binds to 127.0.0.1 and must not be
# exposed to a network while it still uses the default admin password.

POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_USER=ucode
POSTGRES_PASSWORD=ucode_local_dev
POSTGRES_MAX_CONNECTIONS=20

GET_REQUEST_REDIS_HOST=redis
GET_REQUEST_REDIS_PORT=6379
GET_REQUEST_REDIS_PASSWORD=
GET_REQUEST_REDIS_DATABASE=0

MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=ucode
MINIO_SECRET_KEY=ucode_local_dev
MINIO_SSL=false
MINIO_USE_SSL=false
MINIO_PROTOCOL=false
MINIO_BUCKET=ucode
MINIO_FOLDER=files

SECRETS_PROVIDER=redis
VAULT_MOUNT_PATH=ucode
VAULT_SECRET_PATH=local

# Generated for this installation. Changing it invalidates every issued token.
SECRET_KEY=%s

ENVIRONMENT=debug
K8S_NAMESPACE=local

# Host-side ports, offset so the stack does not collide with a Postgres or
# Redis you already run.
UCODE_POSTGRES_PORT=5433
UCODE_REDIS_PORT=6380
UCODE_MINIO_PORT=9010
UCODE_MINIO_CONSOLE_PORT=9011

# Where the browser reaches the backends. Baked into the admin page at
# container start, so they must be addresses the browser can resolve.
UCODE_ADMIN_PORT=3000
UCODE_API_URL=http://127.0.0.1:8000
UCODE_AUTH_URL=http://127.0.0.1:9104
`, secretKey)
}
