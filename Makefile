MODULES := ./services/auth/... ./services/company/... ./services/gateway/... ./services/object-builder/... ./cmd/ucode/...

.PHONY: build vet test test-integration fmt tidy proto swagger cli cli-assets \
        cli-assets-check images smoke release-dry-run

build:
	go build $(MODULES)

vet:
	go vet $(MODULES)

# Unit tests only. Integration tests are behind the `integration` build tag.
test:
	go test $(MODULES)

test-integration:
	go test -tags=integration $(MODULES)

fmt:
	gofmt -l -w services

tidy:
	@for m in services/*/; do (cd "$$m" && go mod tidy); done

# Regenerates every service's Go bindings from its own protos/ directory.
#
# That directory — not the repository root's proto/ — is what the checked-in
# genproto was built from: regenerating all 904 files from it reproduces them
# byte for byte, which is what makes this safe to run. Needs protoc with
# protoc-gen-go and protoc-gen-go-grpc on PATH.
proto:
	@for s in auth company gateway object-builder; do \
		rm -rf "services/$$s/genproto" ; \
		for d in services/$$s/protos/*/ ; do \
			protoc -I="$$d" -I="services/$$s/protos" \
				--go_out="services/$$s" --go-grpc_out="services/$$s" "$$d"*.proto || exit 1 ; \
		done ; \
		printf '  %s\n' "$$s" ; \
	done
	@echo "regenerated from services/*/protos"

# Regenerates the swagger docs the two HTTP services serve at /swagger.
#
# --parseDependency is not optional: the annotations name generated protobuf
# types, and without it swag never opens those packages and stops at the first
# one. --parseInternal is what lets it see this module's own packages.
#
# Output is not byte-for-byte stable — swag names a definition after the
# shortest unambiguous form of its package path, and which of two colliding
# packages gets the short name changes between runs. The spec is equivalent
# either way, so expect noise in the diff and check the paths, not the bytes.
#
# Needs swag v1.8.9, matching the library the services link:
#   go install github.com/swaggo/swag/cmd/swag@v1.8.9
SWAG ?= swag

swagger:
	@for s in auth gateway; do \
		(cd services/$$s && $(SWAG) init -g api/api.go -o api/docs \
			--parseDependency --parseInternal) || exit 1 ; \
	done
	@echo "regenerated services/{auth,gateway}/api/docs"

# The CLI embeds the stack definition so `ucode start` needs no checkout.
# deploy/ stays canonical; this copies it in before building.
cli-assets:
	cp deploy/docker-compose.yml deploy/postgres-init.sql cmd/ucode/assets/

cli: cli-assets
	cd cmd/ucode && go build -o ../../bin/ucode .
	@echo "built bin/ucode"

# Releases run this instead of cli-assets: a release must fail on a stale copy,
# not silently repair it, or the tag would ship something no test ever ran.
cli-assets-check:
	@for f in docker-compose.yml postgres-init.sql; do \
		cmp -s deploy/$$f cmd/ucode/assets/$$f || { \
			echo "cmd/ucode/assets/$$f is out of date with deploy/$$f — run: make cli-assets"; \
			exit 1; }; \
	done
	@echo "cli assets in sync"

# Build every image the local stack runs, under the same names CI publishes so
# the compose file needs no special casing — only a different tag.
IMAGE_PREFIX ?= ghcr.io/ucode-io/ucode-opensource
IMAGE_TAG    ?= local

images:
	docker build -f deploy/Dockerfile.migrate -t $(IMAGE_PREFIX)/migrate:$(IMAGE_TAG) .
	@for s in auth company gateway object-builder; do \
		docker build -f deploy/Dockerfile.service --build-arg SERVICE=$$s -t $(IMAGE_PREFIX)/$$s:$(IMAGE_TAG) . ; \
	done
	docker build -f deploy/Dockerfile.admin -t $(IMAGE_PREFIX)/admin:$(IMAGE_TAG) .
	@echo "built $(IMAGE_PREFIX)/*:$(IMAGE_TAG) — run with UCODE_VERSION=$(IMAGE_TAG)"

# Walk the path a new user takes against a running stack. Needs `ucode start`.
smoke:
	./deploy/smoke-test.sh

# Build the release locally without publishing anything. Needs goreleaser.
release-dry-run:
	goreleaser release --snapshot --clean
