# MockInventory

**Cluster:** [sdlc/testing-infra](README.md) · **Status:** planned · **Severity:** n/a (subcommand)

## Concept

`krit mocks` — enumerate all mocks in the test corpus, what they
target, and whether they're unused.

## Shape

```
$ krit mocks
Total mocks: 147
Unused mock targets: 12
  - Api (UserServiceTest:42, unused)
  - DiskCache (RepositoryTest:18, unused)
```

## Infra reuse

- Reference index scoped to test paths.
- Mocking library registry (mockk, mockito, etc.).

## Links

- Parent: [`../README.md`](../README.md)
- Related: `roadmap/clusters/testing-quality/mock-without-verify.md`
