# AdMobInitializedBeforeConsent

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

`MobileAds.initialize(...)` in `Application.onCreate` without a
preceding `ConsentInformation.requestConsentInfoUpdate(...)`.

## Triggers

```kotlin
class App : Application() {
    override fun onCreate() {
        super.onCreate()
        MobileAds.initialize(this)
    }
}
```

## Does not trigger

```kotlin
class App : Application() {
    override fun onCreate() {
        super.onCreate()
        consentInformation.requestConsentInfoUpdate(...)
        // initialize after consent
    }
}
```

## Dispatch

`call_expression` on `MobileAds.initialize` inside `Application.onCreate`
with no preceding consent call.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
