# DependencyFromBintray

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

Repository URL containing `dl.bintray.com` or `jcenter.bintray.com`.

## Triggers

```kotlin
maven { url = uri("https://dl.bintray.com/example/maven") }
```

## Does not trigger

```kotlin
maven { url = uri("https://repo.example.com/maven") }
```

## Dispatch

String-literal scan inside `maven { url = ... }` blocks.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
