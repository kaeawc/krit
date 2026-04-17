# UntestedPublicApi

**Cluster:** [sdlc/testing-infra](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

Public class or function with no test file referencing it.

## Triggers

`class UserRepository { fun get(id: Long): User }` in
`src/main/`, no `UserRepositoryTest.kt` or any test file mentioning
`UserRepository.get` in `src/test/`.

## Does not trigger

At least one test file references the symbol.

## Dispatch

Cross-file reference index; partition by file path into
test/non-test.

## Links

- Parent: [`../README.md`](../README.md)
