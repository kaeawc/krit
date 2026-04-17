# TestOnlyImportInProduction

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`import com.example.Fake*` / `import org.mockito.*` / `import io.mockk.*`
in a non-test file.

## Triggers

```kotlin
// src/main/.../ProdClass.kt
import io.mockk.mockk
```

## Does not trigger

Same import in a test file.

## Dispatch

Import-header scan with a default test-only prefix list (plus
config-extensible).

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
