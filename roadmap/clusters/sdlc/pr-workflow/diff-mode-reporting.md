# DiffModeReporting

**Cluster:** [sdlc/pr-workflow](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

Given a PR diff, only report findings whose lines intersect with
changed lines. Dramatically reduces CI noise on large existing
codebases.

## Shape

```
$ krit --diff main...HEAD .
src/main/.../Changed.kt:42 MagicNumber ...
```

The underlying dispatcher is unchanged; a post-filter walks
findings and keeps those whose `(file, line)` pair is in the diff.

## Infra reuse

- Existing line-precise findings.
- New: `--diff <ref>` flag that runs `git diff --name-only <ref>`
  and `git diff --unified=0 <ref>` to collect changed lines.

## Recommendation from the survey

> The single highest-leverage target is diff-mode reporting. It needs
> no new static-analysis capability — just a line-range filter over
> the existing finding set — but fundamentally changes how krit
> integrates into CI for large repos.

## Links

- Parent: [`../README.md`](../README.md)
