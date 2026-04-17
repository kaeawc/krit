# FragmentConstructor

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

Fragment subclasses that declare constructors with parameters but are missing a required public no-arg constructor. Android recreates Fragments via reflection using the no-arg constructor; if it is absent the app will crash with `InstantiationException` on process restart or configuration change.

## Example — triggers

```kotlin
class LoginFragment(private val userId: String) : Fragment() {
    // No no-arg constructor — crashes on back-stack restoration
    override fun onCreateView(
        inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?
    ) = inflater.inflate(R.layout.fragment_login, container, false)
}
```

## Example — does not trigger

```kotlin
class LoginFragment : Fragment() {
    private var userId: String? = null

    // Arguments passed via Bundle, not constructor
    companion object {
        fun newInstance(userId: String) = LoginFragment().apply {
            arguments = bundleOf("userId" to userId)
        }
    }

    override fun onCreateView(
        inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?
    ) = inflater.inflate(R.layout.fragment_login, container, false)
}
```

## Implementation notes

- Dispatch: `class_declaration`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Small
- Related: `ViewConstructorRule`, `fragment-constructor` AOSP lint check

## Links

- Parent overview: [`../README.md`](../README.md)
