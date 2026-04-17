# UseValueOf

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

`new Integer(x)`, `new Long(x)`, `new Float(x)`, `new Double(x)`, `new Boolean(x)`, and `new Character(x)` constructor calls in Java interop or Java source files. The constructors are deprecated since Java 9 and removed in Java 17; `Integer.valueOf(x)` and its equivalents cache common values and are the preferred replacement.

## Example — triggers

```kotlin
// Java interop scenario — calling Java code that does this, or in a .java file
val boxed: Integer = Integer(42)          // deprecated constructor
val flag: java.lang.Boolean = Boolean(true) // deprecated constructor
```

## Example — does not trigger

```kotlin
val boxed: Int = 42                    // idiomatic Kotlin, no boxing constructor
val cached: Integer = Integer.valueOf(42)  // explicit valueOf when Java type needed
val flag = true                        // idiomatic Kotlin Boolean
```

## Implementation notes

- Dispatch: `object_creation_expression`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Small
- Related: `JavaPerformanceDetector` (AOSP), `UseValueOf`, auto-fix candidate (replace `new Integer(x)` → `Integer.valueOf(x)`)

## Links

- Parent overview: [`../README.md`](../README.md)
