.PHONY: build test vet lint lint-rules fix schema clean bench integration playground ci regression daemon-verify all install install-completions watch

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o krit ./cmd/krit/
	go build -ldflags "$(LDFLAGS)" -o krit-lsp ./cmd/krit-lsp/
	go build -ldflags "$(LDFLAGS)" -o krit-mcp ./cmd/krit-mcp/
	go build -ldflags "$(LDFLAGS)" -o krit-daemon ./cmd/krit-daemon/
	go build -ldflags "$(LDFLAGS)" -o krit-changelog ./cmd/krit-changelog/

test:
	go test ./... -count=1

vet:
	go vet ./...

# lint-rules enforces static gates against the rules package (all run
# inside TestRulesPackageHasNoCapabilityDrift via the shared Analyze pass):
#   - capability-declaration: ctx.Resolver / .Oracle() needs NeedsResolver / NeedsOracle
#   - concurrent-state: go/WaitGroup/MergeCollectors needs NeedsConcurrent
#   - fix-drift: Fix != FixNone requires an f.Fix assignment in the Check body
#   - opt-in-reason: DefaultActive: false requires an OptInReason classification
#   - java-support-coverage: rules declaring NeedsTypeInfo/NeedsResolver must
#     carry an explicit Java LanguageSupport entry (existing rules are
#     grandfathered; new ones must classify)
lint-rules:
	go test ./internal/ruleslinter/ -run 'TestRulesPackageHasNoCapabilityDrift|TestRulesPackageHasNoNewAdHocCaches|TestRulesPackageHasOptInReasons|TestRulesPackageHasNoDefensiveContextGuards' -count=1
	go test ./internal/rules/ -run 'TestRulesWithTypeInfoDeclareExplicitJavaSupport' -count=1

lint: build
	./krit .

fix: build
	./krit --fix .

schema: build
	./krit --generate-schema > schemas/krit-config.schema.json

clean:
	rm -f krit krit-lsp krit-mcp krit-daemon krit-changelog

bench:
	go test ./internal/... -bench=. -benchmem -count=3 -timeout 120s

integration: build
	bash scripts/integration-test.sh

playground: build
	./krit -f json playground/kotlin-webservice/ | head -20
	./krit -f json playground/android-app/ | head -20

regression: build
	bash scripts/regression-check.sh

# daemon-verify runs the divergence harness unit suite, which is the
# correctness oracle for the daemon's resident-cache path. When daemon
# strict-verify mode lands it will also drive the harness across the
# fixture tree and any opt-in corpus pointed at by KRIT_CORPUS_DIR.
daemon-verify:
	go test ./internal/daemon/ -run 'TestCompare|TestDiff' -count=1

ci: build vet test integration regression daemon-verify

DESTDIR ?=

install: build
	install -d $(DESTDIR)/usr/local/bin
	install -m 755 krit $(DESTDIR)/usr/local/bin/krit
	install -m 755 krit-lsp $(DESTDIR)/usr/local/bin/krit-lsp
	install -m 755 krit-mcp $(DESTDIR)/usr/local/bin/krit-mcp
	install -m 755 krit-daemon $(DESTDIR)/usr/local/bin/krit-daemon

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
