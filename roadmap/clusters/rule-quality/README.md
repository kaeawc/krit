# Rule quality cluster

Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)

## AST-first rewrites

Convert string-matching CheckNode bodies to proper AST walks. Grouped
by urgency: known-FP rules first, then by category.

### Known-FP priority (from multi-repo audit)

- [`ast-rewrite-commit-pref-edits.md`](ast-rewrite-commit-pref-edits.md)
- [`ast-rewrite-commit-transaction.md`](ast-rewrite-commit-transaction.md)
- [`ast-rewrite-check-result.md`](ast-rewrite-check-result.md)
- [`ast-rewrite-double-mutability.md`](ast-rewrite-double-mutability.md)
- [`ast-rewrite-data-class-immutable.md`](ast-rewrite-data-class-immutable.md)

### Potential-bugs category

- [`ast-rewrite-avoid-referential-equality.md`](ast-rewrite-avoid-referential-equality.md)
- [`ast-rewrite-cast-nullable.md`](ast-rewrite-cast-nullable.md)
- [`ast-rewrite-char-array-to-string.md`](ast-rewrite-char-array-to-string.md)
- [`ast-rewrite-exit-outside-main.md`](ast-rewrite-exit-outside-main.md)
- [`ast-rewrite-error-usage-throwable.md`](ast-rewrite-error-usage-throwable.md)

### Style category

- [`ast-rewrite-collapsible-if.md`](ast-rewrite-collapsible-if.md)
- [`ast-rewrite-forbidden-annotation.md`](ast-rewrite-forbidden-annotation.md)
- [`ast-rewrite-forbidden-comment.md`](ast-rewrite-forbidden-comment.md)
- [`ast-rewrite-explicit-collection-access.md`](ast-rewrite-explicit-collection-access.md)
- [`ast-rewrite-also-could-be-apply.md`](ast-rewrite-also-could-be-apply.md)

### Android category

- [`ast-rewrite-android-correctness-batch.md`](ast-rewrite-android-correctness-batch.md)
- [`ast-rewrite-android-source-extra-batch.md`](ast-rewrite-android-source-extra-batch.md)

### Audit pass

- [`ast-rewrite-audit.md`](ast-rewrite-audit.md)

### Vibe cleanup (from 2026-04-16 audit)

- [`manifest-confidence-dedup.md`](manifest-confidence-dedup.md) —
  replace 34× duplicated `Confidence() { return 0.75 }` with named
  constants and a default method on ManifestBase
- [`magic-numbers-and-verbose-comments.md`](magic-numbers-and-verbose-comments.md) —
  extract magic numbers to named constants; remove "what" comments
  that restate code

## Configurable test detection

- [`configurable-test-paths.md`](configurable-test-paths.md)
- [`test-path-config-schema.md`](test-path-config-schema.md)
- [`test-path-migration.md`](test-path-migration.md)
