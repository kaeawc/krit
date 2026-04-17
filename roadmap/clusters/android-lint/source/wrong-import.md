# WrongImport

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

`import android.R` statements in application source files. `android.R` is the framework's own resource class; importing it shadows the application's generated `R` class, causing compilation failures or silently resolving resource IDs to the wrong values. The correct import is the app's own package `R` class (e.g., `import com.example.app.R`).

## Example — triggers

```kotlin
import android.R  // wrong — this is the framework R, not the app's R

class WelcomeFragment : Fragment() {
    override fun onCreateView(
        inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?
    ): View = inflater.inflate(R.layout.fragment_welcome, container, false)
    // R.layout.fragment_welcome resolves against android.R — compile error or wrong layout
}
```

## Example — does not trigger

```kotlin
import com.example.myapp.R  // correct — app's own generated resource class

class WelcomeFragment : Fragment() {
    override fun onCreateView(
        inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?
    ): View = inflater.inflate(R.layout.fragment_welcome, container, false)
}
```

## Implementation notes

- Dispatch: `import_header`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Small — match any `import_header` whose import path is exactly `android.R` or `android.R.*`; auto-fix candidate (replace with the module's own package `R` class if determinable from the file's package declaration)
- Related: `WrongImportDetector` (AOSP)

## Links

- Parent overview: [`../README.md`](../README.md)
