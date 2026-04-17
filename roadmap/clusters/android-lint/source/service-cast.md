# ServiceCast

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

`getSystemService()` calls where the return value is cast to a type that does not match the well-known service constant passed as the argument. For example, casting the result of `getSystemService(Context.WIFI_SERVICE)` to `AudioManager` instead of `WifiManager` compiles fine but throws a `ClassCastException` at runtime.

## Example — triggers

```kotlin
class NetworkMonitor(private val context: Context) {
    fun checkWifi() {
        // WIFI_SERVICE should be cast to WifiManager, not ConnectivityManager
        val cm = context.getSystemService(Context.WIFI_SERVICE) as ConnectivityManager
        val network = cm.activeNetwork
    }
}
```

## Example — does not trigger

```kotlin
class NetworkMonitor(private val context: Context) {
    fun checkWifi() {
        val wm = context.getSystemService(Context.WIFI_SERVICE) as WifiManager
        val info = wm.connectionInfo
    }

    fun checkConnectivity() {
        val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val network = cm.activeNetwork
    }
}
```

## Implementation notes

- Dispatch: `call_expression`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Medium — build a lookup table mapping `Context.*_SERVICE` constants to their expected manager types; match the cast type against the expected type; handle both `as` casts and assignment-with-explicit-type in Kotlin
- Related: `ServiceCastDetector` (AOSP), `getSystemService` type-safety patterns

## Links

- Parent overview: [`../README.md`](../README.md)
