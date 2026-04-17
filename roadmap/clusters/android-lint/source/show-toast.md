# ShowToast

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

`Toast.makeText()` calls whose return value is never passed to `.show()`. The toast is created but never displayed; this is almost always a missing `.show()` call and is a silent no-op from the user's perspective.

## Example — triggers

```kotlin
class SettingsActivity : AppCompatActivity() {
    fun onSaveClicked() {
        saveSettings()
        // .show() is missing — the toast will never appear
        Toast.makeText(this, R.string.settings_saved, Toast.LENGTH_SHORT)
    }
}
```

## Example — does not trigger

```kotlin
class SettingsActivity : AppCompatActivity() {
    fun onSaveClicked() {
        saveSettings()
        Toast.makeText(this, R.string.settings_saved, Toast.LENGTH_SHORT).show()
    }
}

// Storing for later .show() is also acceptable
class DeferredToast(context: Context) {
    private val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    fun display() { toast.show() }
}
```

## Implementation notes

- Dispatch: `call_expression`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Small — detect `Toast.makeText(...)` calls; flag if the result is not immediately chained with `.show()` and not stored in a variable that has a subsequent `.show()` call; simple chained-call pattern covers the majority of cases
- Related: `ToastDetector` (AOSP)

## Links

- Parent overview: [`../README.md`](../README.md)
