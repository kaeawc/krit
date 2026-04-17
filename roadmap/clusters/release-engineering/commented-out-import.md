# CommentedOutImport

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** info · **Default:** active

## Catches

`// import ...` — a commented-out import is either dead or a
half-done refactor.

## Triggers

```kotlin
// import com.example.old.Thing
```

## Does not trigger

```kotlin
import com.example.new.Thing
```

## Dispatch

Line rule.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
