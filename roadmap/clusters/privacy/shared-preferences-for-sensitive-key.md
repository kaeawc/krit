# SharedPreferencesForSensitiveKey

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`prefs.edit().putString("auth_token", ...)` — sensitive keys
should live in `EncryptedSharedPreferences` or the Keystore.

## Triggers

```kotlin
prefs.edit().putString("auth_token", token).apply()
```

## Does not trigger

```kotlin
EncryptedSharedPreferences.create(...)
    .edit().putString("auth_token", token).apply()
```

## Dispatch

`call_expression` on `putString`/`putInt`/`putLong` whose key
literal matches `/token|secret|password|pin|auth/i`.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
