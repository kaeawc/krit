# DependencySnapshotInRelease

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`dependencies { implementation("group:name:1.2.3-SNAPSHOT") }` in
a production module.

## Triggers

```kotlin
implementation("com.example:lib:1.2.3-SNAPSHOT")
```

## Does not trigger

```kotlin
implementation("com.example:lib:1.2.3")
```

## Configuration

```yaml
supply-chain:
  DependencySnapshotInRelease:
    allowedSnapshots:
      - "com.example:gradle-plugin"
      - "org.corp.internal:*"
    suppressUntil: "2025-06-01"
```

`allowedSnapshots` takes coordinate patterns (group:name, wildcards
supported). `suppressUntil` is a date after which the rule re-enables
— for temporary snapshot testing windows.

## Dispatch

String-literal scan inside `dependencies { }` for a `-SNAPSHOT` suffix.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
