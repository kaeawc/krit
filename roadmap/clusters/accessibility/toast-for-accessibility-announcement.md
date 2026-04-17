# ToastForAccessibilityAnnouncement

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`Toast.makeText(...)` followed by `.show()` in a function whose name
contains "accessibility" or "announce" — should use
`AccessibilityManager.interrupt()` / `announceForAccessibility`.

## Triggers

```kotlin
fun announceStatus(msg: String) {
    Toast.makeText(context, msg, Toast.LENGTH_SHORT).show()
}
```

## Does not trigger

```kotlin
fun announceStatus(msg: String) {
    view.announceForAccessibility(msg)
}
```

## Dispatch

`call_expression` on `Toast.makeText` inside a function with a
name-pattern gate.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
