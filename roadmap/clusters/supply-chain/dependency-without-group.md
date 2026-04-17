# DependencyWithoutGroup

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`"junit:4.13"` shorthand — old-style coordinate that omits the
group.

## Triggers

```kotlin
testImplementation("junit:4.13")
```

## Does not trigger

```kotlin
testImplementation("junit:junit:4.13")
```

## Dispatch

Coordinate string-literal scan; count `:` separators.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
