# MockFinalClassWithoutMockMaker

**Cluster:** [testing-quality](README.md) · **Status:** deferred · **Severity:** warning · **Default:** inactive

## Catches

`mockk<Foo>()` / `mock(Foo::class.java)` where `Foo` is a non-open
class in a module without `mockito-inline` / `mockk` final-class
support.

## Triggers

`mockk<FinalClass>()` in a module without the required build
dependency.

## Does not trigger

`Foo` is `open`, or the module has the right dependency.

## Dispatch

Cross-file: resolve mock target class `open` modifier, check
`BuildGraph` for the required test dependency.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
