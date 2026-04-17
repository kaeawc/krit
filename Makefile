.PHONY: build test vet lint fix schema clean bench integration playground ci regression docs all install install-completions watch generate

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X main.version=$(VERSION)

# Regenerate build/rule_inventory.json and the downstream code-generated
# Meta() files. Required before build/test because build/ is gitignored
# and the inventory is a build artifact, not source.
generate:
	python3 tools/rule_inventory.py
	go generate ./internal/rules/...

build: generate
	go build -ldflags "$(LDFLAGS)" -o krit ./cmd/krit/
	go build -ldflags "$(LDFLAGS)" -o krit-lsp ./cmd/krit-lsp/
	go build -ldflags "$(LDFLAGS)" -o krit-mcp ./cmd/krit-mcp/

test: generate
	go test ./... -count=1

vet:
	go vet ./...

lint: build
	./krit .

fix: build
	./krit --fix .

schema: build
	./krit --generate-schema > schemas/krit-config.schema.json

clean:
	rm -f krit krit-lsp krit-mcp

bench:
	go test ./internal/... -bench=. -benchmem -count=3 -timeout 120s

integration: build
	bash scripts/integration-test.sh

playground: build
	./krit -f json playground/kotlin-webservice/ | python3 -m json.tool | head -20
	./krit -f json playground/android-app/ | python3 -m json.tool | head -20

regression: build
	bash scripts/regression-check.sh

docs:
	python3 scripts/github/deploy_pages.py serve

ci: build vet test integration regression

DESTDIR ?=

install: build
	install -d $(DESTDIR)/usr/local/bin
	install -m 755 krit $(DESTDIR)/usr/local/bin/krit
	install -m 755 krit-lsp $(DESTDIR)/usr/local/bin/krit-lsp
	install -m 755 krit-mcp $(DESTDIR)/usr/local/bin/krit-mcp

install-completions:
	install -d $(HOME)/.local/share/bash-completion/completions
	install -m 644 scripts/completions/krit.bash $(HOME)/.local/share/bash-completion/completions/krit
	install -d $(HOME)/.local/share/zsh/site-functions
	install -m 644 scripts/completions/krit.zsh $(HOME)/.local/share/zsh/site-functions/_krit
	install -d $(HOME)/.config/fish/completions
	install -m 644 scripts/completions/krit.fish $(HOME)/.config/fish/completions/krit.fish

watch:
	@echo "Watching for changes... (requires fswatch)"
	@fswatch -o internal/ cmd/ | xargs -n1 -I{} make test 2>/dev/null || \
		echo "Install fswatch: brew install fswatch"

all: build vet test
