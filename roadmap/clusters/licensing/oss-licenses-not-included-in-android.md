# OssLicensesNotIncludedInAndroid

**Cluster:** [licensing](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

Android app module with `implementation` dependencies but no
`com.google.android.gms.oss-licenses-plugin` in its plugin list
and no declared LICENSE file in the app module.

## Triggers

App module with dozens of dependencies and no attribution surface.

## Does not trigger

Plugin applied or LICENSE file present.

## Dispatch

`BuildGraph` plugin check + file presence.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
