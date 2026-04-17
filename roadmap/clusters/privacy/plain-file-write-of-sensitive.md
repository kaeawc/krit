# PlainFileWriteOfSensitive

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`File("credentials").writeText(...)` — plain-file writes to paths
whose name includes credential / token / secret / auth.

## Triggers

```kotlin
File(dir, "credentials.json").writeText(json)
```

## Does not trigger

```kotlin
File(dir, "cache.json").writeText(json)
// Or EncryptedFile.Builder(...).build().writeText(json)
```

## Dispatch

`call_expression` on `writeText`/`writeBytes` whose receiver is a
`File(...)` with a sensitive name.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
