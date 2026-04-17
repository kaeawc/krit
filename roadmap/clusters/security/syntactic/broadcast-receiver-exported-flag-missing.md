# BroadcastReceiverExportedFlagMissing

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`registerReceiver(receiver, filter)` without the
`RECEIVER_EXPORTED` / `RECEIVER_NOT_EXPORTED` flag argument. Required
on API 34+.

## Example — triggers

```kotlin
context.registerReceiver(myReceiver, IntentFilter("com.example.ACTION"))
```

## Example — does not trigger

```kotlin
ContextCompat.registerReceiver(
    context,
    myReceiver,
    IntentFilter("com.example.ACTION"),
    ContextCompat.RECEIVER_NOT_EXPORTED,
)
```

## Implementation notes

- Dispatch: `call_expression` on `registerReceiver` with fewer than
  three arguments, or three arguments where none is a
  `RECEIVER_EXPORTED`/`RECEIVER_NOT_EXPORTED` constant.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: existing `ExportedReceiver` manifest rule
