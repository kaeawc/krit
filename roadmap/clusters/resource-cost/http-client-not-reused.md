# HttpClientNotReused

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

Java 11 `HttpClient.newHttpClient()` called from a method body
without being cached in a field.

## Triggers

```kotlin
fun fetch(url: String): HttpResponse<String> =
    HttpClient.newHttpClient().send(HttpRequest.newBuilder(URI(url)).build(),
        BodyHandlers.ofString())
```

## Does not trigger

```kotlin
private val client = HttpClient.newHttpClient()
fun fetch(url: String): HttpResponse<String> =
    client.send(HttpRequest.newBuilder(URI(url)).build(), BodyHandlers.ofString())
```

## Dispatch

`call_expression` on `HttpClient.newHttpClient()` inside a regular
function body.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
