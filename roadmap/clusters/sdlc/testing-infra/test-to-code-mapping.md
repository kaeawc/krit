# TestToCodeMapping

**Cluster:** [sdlc/testing-infra](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

Resolve `FooTest` → `Foo`, surface tests that exist but don't
actually call the function they claim to cover.

## Shape

```
$ krit test-coverage --static
FooTest.loads: claims to cover Foo.load, but never calls it
```

## Infra reuse

- Naming convention helper (`.replace("Test", "")`).
- Reference index.

## Links

- Parent: [`../README.md`](../README.md)
