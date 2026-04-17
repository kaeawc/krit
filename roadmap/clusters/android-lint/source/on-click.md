# OnClick

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** in-progress ·
**Severity:** warning · **Default:** active

## What it catches

Methods referenced by `android:onClick` XML attributes in layout files that either do not exist in the corresponding Activity/Fragment or have the wrong signature. The required signature is `fun methodName(view: View)` (public, single `View` parameter). A mismatch compiles without error but throws a `NoSuchMethodException` at runtime when the button is tapped.

This rule is currently a stub (`Check()` returns nil). It is blocked on the manifest+layout XML scanner being able to extract `android:onClick` attribute values and map them to the source class that inflates the layout.

## Example — triggers

```kotlin
// Layout XML declares: android:onClick="onSubmitClicked"
// But the Activity has no such method, or it has the wrong signature:
class FormActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
    }

    // Wrong signature — must take a View parameter
    fun onSubmitClicked() {
        submitForm()
    }
}
```

## Example — does not trigger

```kotlin
// Layout XML declares: android:onClick="onSubmitClicked"
class FormActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
    }

    // Correct signature — public fun with exactly one View parameter
    fun onSubmitClicked(view: View) {
        submitForm()
    }
}

// Using programmatic listeners avoids android:onClick entirely — no risk
class ProgrammaticActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_form)
        findViewById<Button>(R.id.submit).setOnClickListener { submitForm() }
    }
}
```

## Implementation notes

- Dispatch: XML attribute (`android:onClick`) + `function_declaration`
- Infra reuse: `internal/android/` (manifest/layout XML scanner), `internal/rules/android_correctness.go` (stub lives here)
- Effort: Medium — requires two-phase analysis: (1) extract `android:onClick` method names from layout XML files via the Android XML scanner; (2) cross-reference against public `fun methodName(view: View)` declarations in source files that inflate those layouts; blocked on the XML scanner exposing attribute values to rules
- Related: `OnClickDetector` (AOSP), manifest+layout XML scanner, `WrongImport`

## Links

- Parent overview: [`../README.md`](../README.md)
