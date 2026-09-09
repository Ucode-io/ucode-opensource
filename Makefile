MODULES := ./services/auth/... ./services/company/... ./services/gateway/... ./services/object-builder/... ./cmd/ucode/...

.PHONY: build vet test test-integration fmt tidy cli cli-assets

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

# The CLI embeds the stack definition so `ucode start` needs no checkout.
# deploy/ stays canonical; this copies it in before building.
cli-assets:
	cp deploy/docker-compose.yml deploy/postgres-init.sql cmd/ucode/assets/

cli: cli-assets
	cd cmd/ucode && go build -o ../../bin/ucode .
	@echo "built bin/ucode"
