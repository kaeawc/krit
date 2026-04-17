# HardcodedHttpUrl

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

String literal matching `^http://` passed as the argument to a known
HTTP builder: `Retrofit.Builder().baseUrl(...)`,
`OkHttpClient` request builders, `URL(...)`, `HttpURLConnection.openConnection`.

## Example — triggers

```kotlin
val retrofit = Retrofit.Builder()
    .baseUrl("http://api.example.com/")
    .build()
```

## Example — does not trigger

```kotlin
val retrofit = Retrofit.Builder()
    .baseUrl("https://api.example.com/")
    .build()

// http://localhost is allowed (see release-engineering/hardcoded-localhost-url.md
// for a separate rule that handles this distinct case)
```

## Implementation notes

- Dispatch: `call_expression` on `baseUrl` / `url` / `URL` / `openConnection`.
- String literal inspection on the single argument.
- Skips `http://localhost` and `http://10.0.2.2` (Android emulator).

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`network-security-config-debug-overrides.md`](network-security-config-debug-overrides.md)
