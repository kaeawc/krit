# JsonPolymorphicUnsafeType

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

Jackson default typing enabled (already caught by tier-1's
[`../syntactic/jackson-default-typing.md`](../syntactic/jackson-default-typing.md))
*and* the deserialized value crosses a trust boundary.

## Shape

```kotlin
val mapper = ObjectMapper().activateDefaultTyping(...)
val obj = mapper.readValue(request.inputStream(), Any::class.java)
```

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
- Related: [`../syntactic/jackson-default-typing.md`](../syntactic/jackson-default-typing.md)
