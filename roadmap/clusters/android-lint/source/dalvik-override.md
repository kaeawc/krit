# DalvikOverride

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** in-progress ·
**Severity:** warning · **Default:** active

## What it catches

Method resolution edge cases where Dalvik's override semantics differed from the Java specification and from ART. Specifically, Dalvik (pre-Android 5.0) resolved overrides on package-private methods from a parent class differently than ART does, which could cause a method call to silently dispatch to the wrong implementation depending on the runtime.

**This rule is effectively obsolete.** Dalvik was removed in Android 5.0 (API 21, released 2014). The current `minSdkVersion` for new apps on the Play Store must be at least API 21 as of 2024. The rule is registered as a stub (`Check()` returns nil) and is a better candidate for removal than for implementation.

## Example — triggers

```kotlin
// Theoretical case — package-private parent method shadowed in subclass
// (Dalvik resolved this differently from ART; ART matches Java spec)
open class Base {
    // package-private in Java terms — no visibility modifier in Kotlin maps to public
    fun compute(): Int = 1
}

class Derived : Base() {
    // In Dalvik, override resolution of package-private methods behaved unexpectedly
    fun compute(): Int = 2  // no 'override' keyword — intentional shadowing
}
```

## Example — does not trigger

```kotlin
open class Base {
    open fun compute(): Int = 1
}

class Derived : Base() {
    override fun compute(): Int = 2  // explicit override — unambiguous on all runtimes
}
```

## Implementation notes

- Dispatch: `function_declaration`
- Infra reuse: `internal/rules/android_correctness.go` (stub lives here)
- Effort: Low (to remove the stub registration) / N/A (to implement — rule has no practical value)
- Related: ART vs Dalvik migration guides; `minSdkVersion` enforcement rules

> **Recommendation:** Remove this rule from the registry rather than implement it. File a separate cleanup task to delete the stub and its registration from `android_correctness.go`.

## Links

- Parent overview: [`../README.md`](../README.md)
