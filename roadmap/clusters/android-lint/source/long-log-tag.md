# LongLogTag

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

`Log.d/i/w/e/v/wtf()` calls where the tag string literal or constant value exceeds 23 characters. On API levels below 26 (Android 8.0), the Android logging system silently truncates tags longer than 23 characters, causing logcat filtering to break. On some older devices the call throws an `IllegalArgumentException` and crashes the app.

## Example — triggers

```kotlin
class NetworkRepositoryImpl {
    companion object {
        // 24 characters — exceeds the 23-character limit
        private const val TAG = "NetworkRepositoryImpl_v2"
    }

    fun fetchUser(id: String) {
        Log.d(TAG, "Fetching user $id")
    }
}
```

## Example — does not trigger

```kotlin
class NetworkRepositoryImpl {
    companion object {
        private const val TAG = "NetworkRepository" // 17 characters — safe
    }

    fun fetchUser(id: String) {
        Log.d(TAG, "Fetching user $id")
    }
}
```

## Implementation notes

- Dispatch: `call_expression`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Small — resolve the tag argument to a string literal or `const val` value, then check `len(value) > 23`
- Related: `LogDetector` (AOSP), `LogTagMismatch`

## Links

- Parent overview: [`../README.md`](../README.md)
