# GsonPolymorphicFromJson

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`Gson().fromJson(..., Any::class.java)` or
`Gson().fromJson(..., Object::class.java)` — polymorphic Gson
deserialization.

## Example — triggers

```kotlin
val any = Gson().fromJson(raw, Any::class.java)
```

## Example — does not trigger

```kotlin
val user = Gson().fromJson(raw, User::class.java)
```

## Implementation notes

- Dispatch: `call_expression` where callee resolves to
  `Gson.fromJson` and the second argument text is `Any::class.java`
  or `Object::class.java`.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`jackson-default-typing.md`](jackson-default-typing.md),
  [`java-object-input-stream.md`](java-object-input-stream.md)
