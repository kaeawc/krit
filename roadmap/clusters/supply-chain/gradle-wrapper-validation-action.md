# GradleWrapperValidationAction

**Cluster:** [supply-chain](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Catches

Project contains a GitHub Actions workflow using `gradle/gradle-build-action`
without a preceding `gradle/wrapper-validation-action` step.

## Triggers

```yaml
jobs:
  build:
    steps:
      - uses: actions/checkout@v4
      - uses: gradle/gradle-build-action@v2
```

## Does not trigger

```yaml
- uses: gradle/wrapper-validation-action@v2
- uses: gradle/gradle-build-action@v2
```

## Dispatch

`.github/workflows/*.yml` scanner (new; cheap YAML parse).

## Links

- Parent: [`roadmap/63-supply-chain-hygiene-rules.md`](../../63-supply-chain-hygiene-rules.md)
