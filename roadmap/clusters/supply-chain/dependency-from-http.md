# DependencyFromHttp

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`maven { url "http://..." }` — plaintext repository fetch.

## Triggers

```kotlin
maven { url = uri("http://internal.example.com/maven") }
```

## Does not trigger

```kotlin
maven { url = uri("https://internal.example.com/maven") }
```

## Configuration

```yaml
supply-chain:
  DependencyFromHttp:
    allowedUrls:
      - "http://artifactory.internal.corp"
      - "http://10.0.0."
```

`allowedUrls` is a prefix match list. Any repository URL starting
with an allowed prefix is skipped. Covers internal JFrog/Nexus/
Artifactory instances behind VPNs where HTTPS isn't configured.

## Dispatch

String-literal scan inside `maven { url = ... }`.

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
