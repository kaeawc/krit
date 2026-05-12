# External Rules

Krit's built-in registry follows the
[rule scope guardrail](rule-scope.md). For rules that don't qualify —
house style, project-specific conventions, opinionated checks — Krit
exposes a public registration package so a downstream project can add
rules without forking the analyzer.

## In-process registration (today)

A consumer that builds Krit from source (or vendors it as a library)
imports [`pkg/extension`](../pkg/extension/extension.go) and calls
`Register` from an `init()` block:

```go
package myteamrules

import (
    "github.com/kaeawc/krit/pkg/extension"
)

func init() {
    extension.Register(&extension.Rule{
        ID:          "MyTeamRule",
        Category:    "myteam",
        Description: "Forbid direct calls to InternalThing",
        Sev:         extension.SeverityWarning,
        NodeTypes:   []string{"call_expression"},
        Maturity:    extension.MaturityExperimental,
        Check: func(ctx *extension.Context) {
            // ...
        },
    })
}
```

Importing the package anywhere in the build (`import _ "myorg/myteamrules"`
in `cmd/krit/main.go`, for example) is enough — Go's `init()` runs the
registration, and the dispatcher picks the rule up alongside the
built-ins.

External rules participate in every analyzer feature on equal footing:

- `Maturity` gating (default-inactive Experimental, opt-in via
  `--experimental` or `experimental: true` in `krit.yml`).
- `RunAfter` ordering, so an external rule can depend on a built-in
  by ID.
- All scope buckets: per-file node, line pass, cross-file, manifest,
  resource, gradle, aggregate.
- Suppression via `@Suppress("MyTeamRule")` and `// krit:ignore[...]`.

The `--list-rules` output, JSON/SARIF emitters, baseline files, and
LSP/MCP servers see externals identically to built-ins.

## Kotlin rule jars (experimental)

Krit now has the first daemon-backed path for Kotlin-authored rule
jars. A rule jar exposes `dev.krit.api.KritRule` through
`META-INF/services/dev.krit.api.KritRule`; the `krit-types` daemon
loads the jar, reads `@KritRuleInfo` metadata, runs the selected rules
per Kotlin file, and returns findings that Krit merges into the normal
JSON/SARIF/baseline output path.

The Gradle plugin also exposes the jar collection as:

```kotlin
krit {
  customRules(project(":build-logic:krit-rules"))
}
```

For local smoke tests without Gradle wiring, build `tools/krit-types`
and pass jars explicitly:

```bash
cd tools/krit-types && ./gradlew shadowJar
krit --custom-rule-jars build-logic/krit-rules/build/libs/krit-rules.jar .
```

The current annotation is named `@KritRuleInfo` because Kotlin cannot
define an annotation class and a ServiceLoader interface both named
`KritRule` in the same package. The ServiceLoader interface keeps the
stable `dev.krit.api.KritRule` name.

## Open design questions for out-of-tree loading

In-process registration requires recompiling Krit's binary. ktlint
takes a different path — JARs discovered via `ServiceLoader`,
loaded with `ktlint -R`. Krit's JVM rule jars now cover that direction
for Kotlin rules, while these broader loader options remain open:

- **Go plugins** (`-buildmode=plugin`): supported on Linux/macOS but
  notoriously brittle (toolchain pinning, race-detector incompatibility,
  no reload on macOS). Not a good fit.
- **Sidecar process**: spawn a rule provider over a stable IPC contract
  (similar to `tools/krit-types/`). Most flexible, biggest implementation
  cost (RPC schema, lifecycle, performance).
- **WASM rule sandbox**: portable, sandboxed, slower than native. Open
  question whether the API surface (`scanner.File`, `FlatTree`, oracle
  facts) maps cleanly to a WASM contract.
- **Embedded scripting** (Starlark, Lua): cheaper to integrate; only
  expresses lexical/AST checks. Insufficient for type-aware rules.

Embedding via `pkg/extension` remains the stable path while the Kotlin
SDK stabilizes.
