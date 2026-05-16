# Suggested fixes

Krit rules carry one of two kinds of edits: a single **autofix** that
`krit --fix` applies automatically, or an ordered list of **suggested
fixes** that a user (or an IDE) picks from. A rule picks exactly one
mode — they are mutually exclusive.

## Autofix vs suggested fix

| Aspect          | Autofix                                | Suggested fix                          |
|-----------------|----------------------------------------|----------------------------------------|
| Slot            | `Rule.Fix` (single `FixLevel`)         | `Rule.SuggestedFixes` (ordered slice)  |
| Applied by      | `krit --fix`, IDE "apply fixes"        | User selection (`krit apply-suggestion`, IDE quick fix) |
| Cardinality     | Exactly one fix per finding            | One or more per finding                |
| User choice     | None — runs unattended                 | Required — the user picks one          |
| JSON surface    | `fixable` + `fixLevel`                 | `suggestedFixes[]`                     |
| Wire schema     | `Fix` on the finding                   | `SuggestedFixes` on the finding        |

Pick **autofix** when there is exactly one correct rewrite and it is
safe to apply unattended at the declared safety tier. Pick
**suggested fixes** when more than one reasonable resolution exists,
or when a "fix" is informational guidance the user must approve.

## Mutual exclusion (enforced at registration)

A rule cannot expose both modes. `api.Register` panics if a rule
declares both `Fix != FixNone` and a non-empty `SuggestedFixes`, and
also panics if the rule's `Implementation` satisfies both
`AutofixRule` and `SuggestedFixRule`.

Validation lives in `Rule.ValidateFixMode` and is covered by:

- Unit tests in `internal/rules/api/fixmode_test.go`.
- A registry-wide gate in `internal/rules/fixmode_registry_test.go`
  that walks every built-in rule and asserts `ValidateFixMode` returns
  nil. Run via `go test ./internal/rules/ -run TestRegistryFixModeIsValid`.

Either gate failing in CI means a rule has mixed declarations or a
malformed `SuggestedFix` entry (empty ID/title, `FixNone` level, or
duplicate ID).

## Order is meaningful

`Rule.SuggestedFixes` is an ordered slice. Position is the
rule-recommended display and application order — list the safer or
stronger fix first, alternatives after. The order survives all
transport layers: registry → finding → JSON → CLI/IDE. Tests pin the
contract end-to-end:

- `internal/rules/api/fixmode_test.go::TestRegister_PreservesSuggestedFixOrdering`
- `internal/output/suggested_fixes_test.go::TestSuggestedFixes_JSONShape`
- `internal/output/suggested_fixes_test.go::TestSuggestedFixes_DeterministicSerialization`
- `editors/intellij-plugin/.../KritJsonParserTest.kt::parse suggested fixes preserves rule-defined order`

Integrations must surface suggestions in the order they appear in the
finding's `suggestedFixes` array. Do not sort by id, title, or safety
tier.

## Authoring example: multiple suggestions

```go
// internal/rules/style/prefer_val.go (illustrative)
var preferValRule = &api.Rule{
    ID:          "PreferVal",
    Category:    "style",
    Description: "Suggest tightening a mutable binding to val.",
    Sev:         api.SeverityWarning,
    NodeTypes:   []string{"property_declaration"},
    // SuggestedFixes is ordered: safer/preferred first.
    SuggestedFixes: []api.SuggestedFix{
        {ID: "use-val", Title: "Convert to val", Level: api.FixSemantic},
        {ID: "explain", Title: "Why prefer val?", Level: api.FixCosmetic},
    },
    Check: func(ctx *api.Context) {
        // ... node analysis omitted ...
        ctx.Emit(&scanner.Finding{
            File: ctx.File.Path, Line: line, Col: col,
            Rule: "PreferVal", RuleSet: "style", Severity: "warning",
            Message: "Prefer val when the binding is read-only.",
            // Suggestions on the finding mirror the rule's declared
            // order. Each entry references the rule's SuggestedFix.ID.
            SuggestedFixes: []scanner.SuggestedFix{
                {
                    ID: "use-val", Title: "Convert to val",
                    Edits: []scanner.SuggestedEdit{
                        {StartLine: line, EndLine: line, Replacement: rewrite},
                    },
                },
                {
                    ID: "explain", Title: "Why prefer val?",
                    Detail:           "var becomes val when the binding is read-only.",
                    ApplicationToken: "help:val-vs-var",
                },
            },
        })
    },
}

func init() { api.Register(preferValRule) }
```

Notes:

- `use-val` is **machine-applicable** — it carries `Edits`. CLI and
  IDE wrappers can apply it directly.
- `explain` is **informational** — no `Edits`, only `Detail` and an
  `ApplicationToken` for the integration to interpret (open a doc,
  show a tooltip, route to a help action).
- Each finding's suggestion ids match a registered
  `Rule.SuggestedFixes[i].ID`, so consumers can correlate per-finding
  edits back to rule metadata (Title, Level).

## Determinism guarantee

Repeated scans of the same source produce byte-identical JSON for
findings with suggestions: same finding ids, same suggestion ids in
the same order, same edit byte ranges and replacement strings. The
cold/warm cache path preserves the same shape — the
`FindingColumns` codec round-trips suggestion data without reordering.
See `TestSuggestedFixes_DeterministicSerialization` and
`TestSuggestedFixes_ColdWarmRoundTrip`.

## JSON shape

```json
{
  "findings": [
    {
      "file": "src/main/kotlin/Example.kt",
      "line": 5, "column": 3,
      "ruleSet": "style", "rule": "PreferVal",
      "severity": "warning",
      "message": "Prefer val when the binding is read-only.",
      "fixable": false,
      "suggestedFixes": [
        {
          "id": "use-val",
          "title": "Convert to val",
          "edits": [
            {"startLine": 5, "endLine": 5, "replacement": "val x = 1"}
          ]
        },
        {
          "id": "explain",
          "title": "Why prefer val?",
          "detail": "var becomes val when the binding is read-only.",
          "applicationToken": "help:val-vs-var"
        }
      ]
    }
  ]
}
```

`fixable` / `fixLevel` track only the autofix slot. A finding can
carry `suggestedFixes` while `fixable` is false (and frequently does).

## Integration guidance

See [Integrations](integrations.md#suggested-fixes-cli-and-ide) for
the `krit apply-suggestion` CLI verb, IDE quick-fix layout, and LSP
notes.
