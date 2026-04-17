# UnsafeDeserialization

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

`ObjectInputStream.readObject`, `Kryo.readObject`, `XStream.fromXML`,
`Gson.fromJson(..., Type)` where the input bytes are tainted.

## Shape

```kotlin
val bytes = request.body().bytes() // untrusted
ObjectInputStream(ByteArrayInputStream(bytes)).readObject()
```

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
- Related: [`../syntactic/java-object-input-stream.md`](../syntactic/java-object-input-stream.md)
