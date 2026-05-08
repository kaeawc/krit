# Rule Scope

Krit ships an opinionated but bounded built-in rule set. This page describes
what qualifies for the built-in registry and what should live elsewhere — as a
project-local rule, a custom rule set, or simply not as a Krit rule at all.

The intent mirrors ktlint's "Coding conventions" guardrail: keep the default
rule set predictable enough that turning Krit on doesn't drown a project in
noise, while leaving a clear path for opinionated checks that real teams want.

## What qualifies for the built-in registry

A rule belongs in `internal/rules/` (and `api.Registry`) if **all** of the
following are true:

1. **It enforces a documented standard.** Examples that qualify:
   - The official Kotlin coding conventions
     (https://kotlinlang.org/docs/coding-conventions.html).
   - The Android Kotlin style guide
     (https://developer.android.com/kotlin/style-guide).
   - Android lint rule families (correctness, performance, security, a11y,
     i18n, manifest, gradle) where Android documents the underlying constraint.
   - A Compose, Coroutines, Flow, or Jetpack guideline published by Google.
   - A documented anti-pattern with a clear primary source — e.g. a JEP, a
     CVE, an Android release note, or a published runtime bug.

2. **It detects something a reasonable team agrees is a problem.** A rule that
   would generate split votes on review (style preference, micro-optimization,
   "house style") is opinionated, not standard — see the next section.

3. **It is implementable with high confidence at the rule's declared
   `Confidence` tier.** If the rule needs to be `ConfidenceLow` to avoid
   false-positive floods on real code, it is a candidate for opt-in
   (`DefaultActive=false`) or `MaturityExperimental` while it soaks. If even
   the high-confidence form would still misfire on common idioms, it does not
   yet belong in the registry.

4. **The rule's evidence is local enough to run in Krit's hot path.** Rules
   that need cross-file analysis must declare `NeedsCrossFile`,
   `NeedsParsedFiles`, etc. Rules that need full type resolution should
   prefer `NeedsResolver`; rules that genuinely require Kotlin Analysis API
   facts must declare `NeedsOracle` and an `OracleCallTargetFilter` /
   `OracleDeclarationProfile` so the workload stays bounded.

5. **Positive and negative fixtures exist** under `tests/fixtures/` and the
   regression discipline in `CLAUDE.md` ("Rule Implementation Guardrails")
   has been followed.

## What does NOT qualify

These typically belong outside the built-in registry:

- **Personal or house style.** "Use trailing lambdas everywhere", "no `var`",
  "no `apply{}`", "always brace single-line ifs". Real teams disagree; ship
  these as a project-local custom rule set.
- **Project-specific naming or layout.** "All ViewModels live under
  `feature/*/vm/`". Use a config-driven rule (e.g. `MissingTestSuffix`,
  module-template config) or a custom rule set, not a built-in.
- **Performance micro-optimizations without a published source.** Without a
  benchmark or a vendor recommendation it's a preference.
- **Checks that require running the program.** Krit is a static analyzer; it
  does not execute code, replace tests, or substitute for runtime sanitizers.
- **Checks that duplicate the Kotlin compiler.** If `kotlinc` already errors
  on it, Krit shouldn't.

## Maturity and graduation

New rules ship via `MaturityExperimental` (see
[`internal/rules/api/rule.go`](../internal/rules/api/rule.go)) so users opt
in via `--experimental` or `experimental: true` in `krit.yml`. Once a rule
has soaked across at least one minor release with no false-positive bug
reports it can graduate to `MaturityStable`.

Rules being removed pass through `MaturityDeprecated` for one minor release.
Deprecated rules are NOT re-enabled by `--experimental` or `--all-rules`;
users who still want them must opt in by name via `--enable-rules <ID>`.

## When to propose a rule out-of-tree

If a rule is valuable to your team but does not pass the qualification
checklist above, prefer one of:

- **Project-local config.** Many rules already accept `Options` that narrow
  their behavior; check the rule's `Meta()` descriptor first.
- **A custom rule set.** Krit is intentionally decoupled from rule
  implementations through `api.Rule` so a downstream project can register
  additional rules without forking the built-in set. (See the roadmap entry
  for pluggable external rule sets — at the time of writing, this surface
  is in design.)
- **Open an issue.** If you believe a rule belongs in the built-in registry
  but the case is not obvious, open an issue with the standard it enforces
  and a few real-world false-positive scenarios you've considered. Reviewers
  will use the checklist above to evaluate.
