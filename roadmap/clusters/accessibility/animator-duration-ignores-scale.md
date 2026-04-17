# AnimatorDurationIgnoresScale

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`ValueAnimator.setDuration(...)` / `ObjectAnimator.setDuration(...)`
without referencing `Settings.Global.ANIMATOR_DURATION_SCALE` —
users who disable animations for motion sensitivity still see the
animation.

## Triggers

```kotlin
ValueAnimator.ofFloat(0f, 1f).apply { duration = 300 }.start()
```

## Does not trigger

```kotlin
val scale = Settings.Global.getFloat(
    contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f,
)
ValueAnimator.ofFloat(0f, 1f).apply { duration = (300 * scale).toLong() }.start()
```

## Dispatch

`call_expression` on `setDuration`/`duration =` with a receiver that
resolves to an Animator type; file-level check for
`ANIMATOR_DURATION_SCALE` reference.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
