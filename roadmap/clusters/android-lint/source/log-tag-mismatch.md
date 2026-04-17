# LogTagMismatch

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

`Log.*()` calls where the tag string does not match the simple name of the enclosing class. Mismatched tags make logcat filtering unreliable and are a common copy-paste error when adapting logging boilerplate from another class.

## Example — triggers

```kotlin
class PaymentFragment : Fragment() {
    companion object {
        // Copied from another class and not updated
        private const val TAG = "CheckoutActivity"
    }

    fun processPayment() {
        Log.d(TAG, "Processing payment")  // tag doesn't match "PaymentFragment"
    }
}
```

## Example — does not trigger

```kotlin
class PaymentFragment : Fragment() {
    companion object {
        private const val TAG = "PaymentFragment"
        // Or the idiomatic form:
        // private val TAG = PaymentFragment::class.java.simpleName
    }

    fun processPayment() {
        Log.d(TAG, "Processing payment")
    }
}
```

## Implementation notes

- Dispatch: `call_expression`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Small — resolve the tag value, walk up the AST to the enclosing `class_declaration`, compare against the class simple name; also accept `ClassName::class.java.simpleName` as a valid pattern
- Related: `LogDetector` (AOSP), `LongLogTag`

## Links

- Parent overview: [`../README.md`](../README.md)
