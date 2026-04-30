# OnClick

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** shipped ·
**Severity:** warning · **Default:** active

## What it catches

Methods referenced by `android:onClick` XML attributes in layout files that either do not exist in the corresponding Activity/Fragment or have the wrong signature. The required signature is `fun methodName(view: View)` (public, single `View` parameter). A mismatch compiles without error but throws a `NoSuchMethodException` at runtime when the button is tapped.

Krit implements this by extracting `android:onClick` handler names from layout
resources and checking source classes that inflate those layouts.

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

- Dispatch: resource-indexed `android:onClick` attributes plus
  `class_declaration` source analysis
- Infra reuse: `internal/android/`, `internal/rules/android_onclick.go`, and
  `internal/rules/android_resource_values.go`
- Coverage: unit tests cover missing handlers, wrong signatures, private
  handlers, wrong parameter types, valid handlers, and classes that do not
  inflate the referenced layout
- Related: `OnClickDetector` (AOSP), manifest+layout XML scanner, `WrongImport`

## Links

- Parent overview: [`../README.md`](../README.md)
