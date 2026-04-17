# VerifyWithoutMock

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`verify(someVar)` / `verify { someVar.x() }` where `someVar` is
not a mock — test is verifying a real instance.

## Triggers

```kotlin
val real = RealApi()
val repo = Repo(real)
repo.load()
verify { real.get() } // MockK throws, but krit catches earlier
```

## Does not trigger

`verify` call where the receiver is known to be a mock.

## Dispatch

`call_expression` on `verify` walking back to the declaration of
its subject.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
