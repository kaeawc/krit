# NonInternationalizedSms

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

`SmsManager.sendTextMessage()` or `SmsManager.sendMultipartTextMessage()` calls where the destination address argument is a string literal that does not begin with a `+` (international dialing prefix). Domestic-format numbers (e.g., `"5551234567"`) fail silently or deliver to the wrong recipient when the device is roaming or the user is in a different country.

## Example — triggers

```kotlin
fun sendVerificationCode(phoneNumber: String) {
    val sms = SmsManager.getDefault()
    // Domestic format — will fail for international users
    sms.sendTextMessage("5551234567", null, "Your code: 1234", null, null)
}
```

## Example — does not trigger

```kotlin
fun sendVerificationCode(phoneNumber: String) {
    val sms = SmsManager.getDefault()
    // International E.164 format
    sms.sendTextMessage("+15551234567", null, "Your code: 1234", null, null)
}

// Dynamic values are not flagged — checked at runtime instead
fun sendTo(internationalNumber: String) {
    SmsManager.getDefault().sendTextMessage(internationalNumber, null, body, null, null)
}
```

## Implementation notes

- Dispatch: `call_expression`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Small — match `sendTextMessage` / `sendMultipartTextMessage` on a `SmsManager` receiver; flag only when the first argument is a string literal not starting with `+`
- Related: `NonInternationalizedSmsDetector` (AOSP)

## Links

- Parent overview: [`../README.md`](../README.md)
