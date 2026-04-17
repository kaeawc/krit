# JacksonDefaultTyping

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`ObjectMapper().enableDefaultTyping(...)` / `.activateDefaultTyping(...)`
— Jackson default typing is a known deserialization-gadget surface.

## Example — triggers

```kotlin
val mapper = ObjectMapper().activateDefaultTyping(LaissezFaireSubTypeValidator.instance)
```

## Example — does not trigger

```kotlin
val mapper = ObjectMapper()
```

## Implementation notes

- Dispatch: `call_expression` on `enableDefaultTyping` /
  `activateDefaultTyping` whose receiver resolves to `ObjectMapper`.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`gson-polymorphic-from-json.md`](gson-polymorphic-from-json.md)
