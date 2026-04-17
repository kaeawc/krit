# MdcPutNoRemove

**Cluster:** [observability](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`MDC.put("key", value)` inside a function with no matching
`MDC.remove("key")` or `MDCCloseable` — leaks across requests.

## Triggers

```kotlin
fun handle(req: Request) {
    MDC.put("reqId", req.id)
    process(req)
}
```

## Does not trigger

```kotlin
fun handle(req: Request) {
    MDC.putCloseable("reqId", req.id).use { process(req) }
}
```

## Dispatch

`call_expression` on `MDC.put` without matching `MDC.remove` in
the same function.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
