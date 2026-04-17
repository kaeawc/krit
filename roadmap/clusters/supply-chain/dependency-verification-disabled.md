# DependencyVerificationDisabled

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`gradle.properties` contains `org.gradle.dependency.verification=off`
or `lenient`.

## Triggers

```properties
org.gradle.dependency.verification=off
```

## Does not trigger

No entry, or `strict`.

## Configuration

```yaml
supply-chain:
  DependencyVerificationDisabled:
    allowLenient: true
```

`allowLenient` (default false) downgrades `lenient` from a finding
to clean. Teams with internal registries that don't publish checksums
can set this while keeping `off` flagged. When false, both `off` and
`lenient` trigger.

## Dispatch

`.properties` file parse.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
