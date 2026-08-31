VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

LDFLAGS := -ldflags "-X github.com/JayveerPrajapati/blueprint/internal/blueprint/version.Version=$(VERSION)"

.PHONY: build install test test-gates vet fmt fmt-check lint mcpb

build:
	go build $(LDFLAGS) -o bin/blueprint ./cmd/blueprint
	go build $(LDFLAGS) -o bin/blueprint-mcp ./cmd/blueprint-mcp

install:
	go install $(LDFLAGS) ./cmd/blueprint ./cmd/blueprint-mcp

test:
	go test -count=1 ./internal/... ./cmd/blueprint-mcp

# Resolve kern from the local install chain (PATH -> ~/.local/bin/kern ->
# ../kern/bin/kern). If none exists the gate suites skip gracefully.
test-gates:
	@if command -v kern >/dev/null 2>&1; then \
		echo "Using kern from PATH: $$(command -v kern)"; \
		KERN_BINARY="$$(command -v kern)" go test -count=1 -timeout 600s ./cmd/blueprint ./internal/blueprint/adapters/kern ./internal/blueprint/metrics; \
	elif [ -x "$$HOME/.local/bin/kern" ]; then \
		echo "Using kern from ~/.local/bin/kern"; \
		KERN_BINARY="$$HOME/.local/bin/kern" go test -count=1 -timeout 600s ./cmd/blueprint ./internal/blueprint/adapters/kern ./internal/blueprint/metrics; \
	elif [ -x "../kern/bin/kern" ]; then \
		echo "Using kern from ../kern/bin/kern"; \
		KERN_BINARY="$$(cd .. && pwd)/kern/bin/kern" go test -count=1 -timeout 600s ./cmd/blueprint ./internal/blueprint/adapters/kern ./internal/blueprint/metrics; \
	else \
		echo "kern not found (PATH, ~/.local/bin, ../kern/bin) — gate tests will skip"; \
		go test -count=1 -timeout 600s ./cmd/blueprint ./internal/blueprint/adapters/kern ./internal/blueprint/metrics; \
	fi

.PHONY: mcpb

# Build a .mcpb bundle for MCP registry distribution (requires `mcpb` CLI or manual zip).
# The bundle wraps bin/blueprint-mcp with a stdio transport manifest.
mcpb: build
	@mkdir -p dist
	@echo '{"manifest":{"name":"blueprint","version":"$(VERSION)","transport":{"type":"stdio"},"command":"blueprint-mcp"}}' > dist/manifest.json
	@cp bin/blueprint-mcp dist/
	@cd dist && zip -j blueprint-mcp-$(VERSION).mcpb manifest.json blueprint-mcp
	@echo "Built dist/blueprint-mcp-$(VERSION).mcpb"
	@echo "SHA256: $$(openssl dgst -sha256 dist/blueprint-mcp-$(VERSION).mcpb | awk '{print $$2}')"
	@rm dist/manifest.json dist/blueprint-mcp

vet:
	go vet ./...

fmt:
	find ./cmd ./internal -name '*.go' -exec gofmt -w {} +

fmt-check:
	test -z "$$(gofmt -l ./cmd ./internal | tee /dev/stderr)"

lint: vet fmt-check