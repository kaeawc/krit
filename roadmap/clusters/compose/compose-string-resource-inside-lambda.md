# ComposeStringResourceInsideLambda

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

`stringResource(...)` inside a non-composable lambda (onClick,
callback) — `stringResource` is composition-only and will crash.

## Triggers

```kotlin
Button(onClick = {
    Log.d("TAG", stringResource(R.string.click_label))
}) { Text("Click") }
```

## Does not trigger

```kotlin
val label = stringResource(R.string.click_label)
Button(onClick = { Log.d("TAG", label) }) { Text("Click") }
```

## Dispatch

`call_expression` on `stringResource` whose nearest enclosing
lambda is not itself annotated `@Composable`.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
