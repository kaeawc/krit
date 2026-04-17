# ImplicitPendingIntent

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`PendingIntent.getActivity` / `.getBroadcast` / `.getService` whose
flag argument does not include `FLAG_IMMUTABLE` or `FLAG_MUTABLE`.
Required on API 31+.

## Example — triggers

```kotlin
PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT)
```

## Example — does not trigger

```kotlin
PendingIntent.getBroadcast(
    context, 0, intent,
    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
)
```

## Implementation notes

- Dispatch: `call_expression` on `PendingIntent.get{Activity,Broadcast,Service}`.
- Inspect the flag argument for the immutable/mutable constants.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`broadcast-receiver-exported-flag-missing.md`](broadcast-receiver-exported-flag-missing.md)
