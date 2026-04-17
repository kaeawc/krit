# BiometricAuthNotFallingBackToDeviceCredential

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`BiometricPrompt.authenticate(...)` with no `setDeviceCredentialAllowed`
or `setAllowedAuthenticators(DEVICE_CREDENTIAL, BIOMETRIC_STRONG)`.
Locks out users without biometric enrollment.

## Triggers

```kotlin
BiometricPrompt(activity, executor, callback)
    .authenticate(PromptInfo.Builder().setTitle("Unlock").build())
```

## Does not trigger

```kotlin
val promptInfo = PromptInfo.Builder()
    .setAllowedAuthenticators(BIOMETRIC_STRONG or DEVICE_CREDENTIAL)
    .setTitle("Unlock")
    .build()
```

## Dispatch

`call_expression` on `BiometricPrompt.authenticate` without the
allowed-authenticators builder call.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
