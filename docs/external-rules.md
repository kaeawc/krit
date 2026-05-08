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

## Open design questions for true out-of-tree loading

In-process registration requires recompiling Krit's binary. ktlint
takes a different path — JARs discovered via `ServiceLoader`,
loaded with `ktlint -R`. For Krit, every option has trade-offs:

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

Until one of these lands, embedding via `pkg/extension` is the
supported path. Open a discussion on the repo if your project has a
strong opinion on which loader to invest in next.
