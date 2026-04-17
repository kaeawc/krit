# GetSignatures

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

Calls to `PackageManager.getPackageInfo()` that pass `PackageManager.GET_SIGNATURES` as a flag. On API levels below 28 this flag is vulnerable to a package fraud attack (a malicious app can insert its own certificate into the returned array). The safe replacement is `GET_SIGNING_CERTIFICATES` (API 28+) with a runtime fallback.

## Example — triggers

```kotlin
fun isTrustedPackage(pm: PackageManager, packageName: String): Boolean {
    val info = pm.getPackageInfo(packageName, PackageManager.GET_SIGNATURES)
    return info.signatures?.any { it.toCharsString() == KNOWN_SIG } == true
}
```

## Example — does not trigger

```kotlin
fun isTrustedPackage(pm: PackageManager, packageName: String): Boolean {
    return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
        val info = pm.getPackageInfo(packageName, PackageManager.GET_SIGNING_CERTIFICATES)
        info.signingInfo?.apkContentsSigners?.any { it.toCharsString() == KNOWN_SIG } == true
    } else {
        @Suppress("DEPRECATION")
        val info = pm.getPackageInfo(packageName, PackageManager.GET_SIGNATURES)
        info.signatures?.any { it.toCharsString() == KNOWN_SIG } == true
    }
}
```

## Implementation notes

- Dispatch: `call_expression`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Small
- Related: `GetSignaturesDetector` (AOSP), `PackageManagerCompat` usage patterns

## Links

- Parent overview: [`../README.md`](../README.md)
