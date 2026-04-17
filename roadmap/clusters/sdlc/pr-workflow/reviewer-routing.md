# ReviewerRouting

**Cluster:** [sdlc/pr-workflow](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

Read `CODEOWNERS`, resolve changed declarations to owners, suggest
reviewers beyond the file-level rules.

## Shape

```
$ krit suggest-reviewers --base main
Suggested reviewers: @team-core (UserRepository), @team-ui (LoginScreen)
```

## Infra reuse

- Cross-file reference index for symbol → file lookup.
- New: `CODEOWNERS` parser.

## Links

- Parent: [`../README.md`](../README.md)
