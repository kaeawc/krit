# OkHttpClientCreatedPerCall

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`OkHttpClient.Builder().build()` inside a non-`object`/non-`@Singleton`
function — disables connection pooling.

## Triggers

```kotlin
fun fetch(url: String): Response {
    val client = OkHttpClient.Builder().build()
    return client.newCall(Request.Builder().url(url).build()).execute()
}
```

## Does not trigger

```kotlin
@Singleton
class HttpClientProvider @Inject constructor() {
    val client = OkHttpClient.Builder().build()
}
```

## Dispatch

`call_expression` chain match inside a regular function body.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
