.PHONY: build test vet lint lint-rules fix schema clean bench integration playground ci regression all install install-completions watch

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o krit ./cmd/krit/
	go build -ldflags "$(LDFLAGS)" -o krit-lsp ./cmd/krit-lsp/
	go build -ldflags "$(LDFLAGS)" -o krit-mcp ./cmd/krit-mcp/

test:
	go test ./... -count=1

vet:
	go vet ./...

# lint-rules enforces three static gates against the rules package:
#   1. capability-declaration: any rule whose Check body calls ctx.Resolver
#      or (*CompositeResolver).Oracle() must declare the matching
#      NeedsResolver / NeedsOracle / NeedsTypeInfo capability.
#   2. concurrent-state: rules using sync.WaitGroup, scanner.MergeCollectors,
#      or `go` statements must declare NeedsConcurrent (and vice versa).
#   3. fix-drift: any rule that declares Fix != FixNone must actually
#      assign a Fix to the finding it emits.
lint-rules:
	go test ./internal/ruleslinter/ -run 'TestRulesPackageHasNoCapabilityDrift|TestRulesPackageHasNoNewAdHocCaches' -count=1

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
	./krit -f json playground/kotlin-webservice/ | head -20
	./krit -f json playground/android-app/ | head -20

regression: build
	bash scripts/regression-check.sh

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
