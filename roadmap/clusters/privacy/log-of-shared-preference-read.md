# LogOfSharedPreferenceRead

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Log.d("TAG", prefs.getString("authToken", ...))` — logs sensitive
data read directly from prefs.

## Triggers

```kotlin
Log.d("Auth", prefs.getString("authToken", null) ?: "")
```

## Does not trigger

```kotlin
Log.d("Auth", "token loaded")
```

## Dispatch

`call_expression` on logger methods whose argument is a
`getString`/`getInt` call on a SharedPreferences receiver with a
sensitive key.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
- Related: [`shared-preferences-for-sensitive-key.md`](shared-preferences-for-sensitive-key.md)
