# LocationBackgroundWithoutRationale

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

Request for `ACCESS_BACKGROUND_LOCATION` permission without a
`shouldShowRequestPermissionRationale` call in the same file.

## Triggers

```kotlin
requestPermissions(arrayOf(Manifest.permission.ACCESS_BACKGROUND_LOCATION), REQ)
```

## Does not trigger

Same code preceded by a rationale dialog flow.

## Dispatch

`call_expression` on `requestPermissions` with the background
location constant; file-level check for
`shouldShowRequestPermissionRationale`.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
