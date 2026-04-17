# PrintlnInProduction

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`println(...)` / `print(...)` / `System.out.println(...)` /
`System.err.println(...)` in a non-test file, outside a top-level
`fun main(...)`.

## Triggers

```kotlin
class UserService {
    fun load() {
        println("loading") // in production source
    }
}
```

## Does not trigger

```kotlin
fun main() { println("script entry point") }
```

## Dispatch

`call_expression` on `println`/`print` gated on `isTestFile` and
enclosing-function-is-main check.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
