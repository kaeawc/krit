# AssertNullableWithNotNullAssertion

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`assertEquals(expected, actual!!)` — `!!` throws before the
assertion can run, losing context.

## Triggers

```kotlin
assertEquals("x", maybeX!!)
```

## Does not trigger

```kotlin
assertNotNull(maybeX)
assertEquals("x", maybeX)
```

## Dispatch

Assertion `call_expression` whose argument contains `!!`.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
