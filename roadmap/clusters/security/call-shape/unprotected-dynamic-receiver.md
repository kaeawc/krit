# UnprotectedDynamicReceiver

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`registerReceiver(receiver, filter, null, handler)` where the
permission argument is null and the intent filter action is an
exported/public action.

## Triggers

```kotlin
context.registerReceiver(
    receiver,
    IntentFilter(Intent.ACTION_SCREEN_ON),
    null,
    null,
)
```

## Does not trigger

```kotlin
context.registerReceiver(
    receiver,
    IntentFilter(Intent.ACTION_SCREEN_ON),
    "com.example.permission.SCREEN_BROADCAST",
    null,
)
```

## Dispatch

`call_expression` on `registerReceiver` with 4 args; check the
permission arg. Distinct from the syntactic
`broadcast-receiver-exported-flag-missing` rule (which is about the
API-34 export flag, not the permission string).

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`../syntactic/broadcast-receiver-exported-flag-missing.md`](../syntactic/broadcast-receiver-exported-flag-missing.md)
