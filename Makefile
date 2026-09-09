MODULES := ./services/auth/... ./services/company/... ./services/gateway/... ./services/object-builder/...

.PHONY: build vet test test-integration fmt tidy

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
