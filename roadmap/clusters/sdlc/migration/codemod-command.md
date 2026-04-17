# CodemodCommand

**Cluster:** [sdlc/migration](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

`krit transform <recipe>` — run an AST rewrite across the whole
repo. A recipe is a named before/after AST snippet pair.

## Shape

```
$ krit transform replace-legacy-timber --apply
# internally: for each file, match before-pattern, rewrite to after
```

Recipe file:

```yaml
# recipes/replace-legacy-timber.yml
name: replace-legacy-timber
before: |
  Timber.d($msg, $*args)
after: |
  logger.debug($msg, *arrayOf($*args))
```

## Infra reuse

- Existing binary-autofix engine.
- New: recipe format + a tree-sitter pattern matcher that binds
  capture variables (`$name`, `$*args`).

## Links

- Parent: [`../README.md`](../README.md)
- Related: [`api-migration-assist.md`](api-migration-assist.md)
