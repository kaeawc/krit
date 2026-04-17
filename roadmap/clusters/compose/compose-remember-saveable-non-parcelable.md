# ComposeRememberSaveableNonParcelable

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

`rememberSaveable { SomeType() }` where `SomeType` isn't `@Parcelize`
or a primitive/String and no `Saver` is passed.

## Triggers

```kotlin
val state = rememberSaveable { MyState() } // MyState is plain data class
```

## Does not trigger

```kotlin
@Parcelize data class MyState(val x: Int) : Parcelable
val state = rememberSaveable { MyState(0) }
```

## Dispatch

`call_expression` on `rememberSaveable` with no `stateSaver` / `saver`
argument, inspecting the returned type.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
