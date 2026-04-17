# TimberTreeNotPlanted

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

Project has Timber usages but no `Timber.plant(...)` call reachable
from an `Application.onCreate`.

## Triggers

`Timber.d("foo")` across the project but no `Timber.plant(...)`
anywhere.

## Does not trigger

```kotlin
class App : Application() {
    override fun onCreate() {
        super.onCreate()
        if (BuildConfig.DEBUG) Timber.plant(Timber.DebugTree())
    }
}
```

## Dispatch

Cross-file aggregation: detect any `Timber.<level>` usage, then
verify at least one `Timber.plant(...)` call-site exists.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
