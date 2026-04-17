# RetrofitCreateInHotPath

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Retrofit.Builder()...create(Api::class.java)` inside a function
that isn't an init / builder — Retrofit proxy creation is
expensive and should be cached.

## Triggers

```kotlin
fun load(): Foo {
    val api = Retrofit.Builder().baseUrl(URL).build().create(Api::class.java)
    return api.get()
}
```

## Does not trigger

```kotlin
private val api = Retrofit.Builder()...create(Api::class.java)
```

## Dispatch

`call_expression` chain inside a non-init function body.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
