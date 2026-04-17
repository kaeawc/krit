# OkHttpCallExecuteSync

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`okHttpClient.newCall(...).execute()` inside a coroutine or suspend
function — blocks the thread.

## Triggers

```kotlin
suspend fun fetch(url: String): Response =
    client.newCall(Request.Builder().url(url).build()).execute()
```

## Does not trigger

```kotlin
suspend fun fetch(url: String): Response =
    client.newCall(Request.Builder().url(url).build()).await() // coroutines-jdk8
```

## Dispatch

`call_expression` on `Call.execute` inside a `suspend fun` or
coroutine lambda.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
