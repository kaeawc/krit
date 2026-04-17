# HardcodedLocalhostUrl

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

URL literal starting with `http://localhost` or `http://10.0.2.2`
in a non-test non-debug source file.

## Triggers

```kotlin
val api = Retrofit.Builder().baseUrl("http://localhost:8080").build()
```

## Does not trigger

Same line in `src/debug/` or inside `if (BuildConfig.DEBUG)`.

## Dispatch

String-literal scan with path and DEBUG-guard checks.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
- Related: `roadmap/clusters/security/syntactic/hardcoded-http-url.md`
