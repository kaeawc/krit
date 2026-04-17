# JavaObjectInputStream

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`ObjectInputStream(...).readObject()` outside test sources. Java
serialization is a known deserialization-gadget surface.

## Example — triggers

```kotlin
ObjectInputStream(FileInputStream(path)).use { it.readObject() }
```

## Example — does not trigger

```kotlin
// JSON, protobuf, or any structured format
val user: User = json.decodeFromString(raw)
```

## Implementation notes

- Dispatch: `call_expression` matching `ObjectInputStream` constructor.
- Test-file skip via `isTestFile(file.Path)`.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`gson-polymorphic-from-json.md`](gson-polymorphic-from-json.md),
  [`jackson-default-typing.md`](jackson-default-typing.md)
