# Precompile rule fixtures

Fixtures for compiler-class diagnostics under `Category: precompile`.

## Layout

```
tests/fixtures/precompile/
  positive/<RuleID>/<n>.kt   -> must produce >=1 finding
  negative/<RuleID>/<n>.kt   -> must produce 0 findings
  fixable/<RuleID>/<n>.kt    -> must produce >=1 finding with a Fix payload
```

`<RuleID>` is the full rule identifier including the `K####-` prefix,
e.g. `K0101-UnreachableCode`. Fixture filenames are conventional,
typically `1.kt`, `2.kt`, ..., but any `.kt` file under the rule directory
is picked up.

## Running

```
go test ./internal/rules/ -run TestPrecompileFixtures -count=1
```

## Conventions

- Each rule should have >=3 positive, >=3 negative fixtures. Fixable rules
  add >=1 fixable fixture.
- Negative fixtures should cover the obvious "looks similar but isn't"
  shapes, e.g. `when` with an explicit `else`, `return` followed by a
  comment block, etc.
- Fixtures must compile under standard kotlinc unless they are
  positive examples of a compile-time error the rule catches. In that
  case, add a leading comment `// EXPECTED-KOTLINC-ERROR: <diagnostic>`
  documenting which kotlinc diagnostic this fixture mirrors. The
  taxonomy at `docs/precompile/taxonomy.md` lists the canonical analog
  per rule.
- Keep fixtures minimal. A 5-line fixture that exercises a single
  branch beats a 50-line fixture covering many.

## Adding fixtures for a new rule

1. Add the rule to `docs/precompile/taxonomy.md` with its `K####` code.
2. Implement the rule under `internal/rules/precompile_<name>.go`.
3. Create `positive/<RuleID>/`, `negative/<RuleID>/`, and (if fixable)
   `fixable/<RuleID>/` directories.
4. Drop in fixtures.
5. Run `go test ./internal/rules/ -run TestPrecompileFixtures -count=1`
   to validate.
