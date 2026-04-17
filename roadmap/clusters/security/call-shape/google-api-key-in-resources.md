# GoogleApiKeyInResources

**Cluster:** [security/call-shape](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`<string>` resource whose name contains `api_key` / `api.key` and
whose value does not start with `@string/`.

## Triggers

```xml
<string name="google_maps_api_key">AIzaSyExampleApiKeyValue1234567890</string>
```

## Does not trigger

```xml
<string name="google_maps_api_key">@string/injected_at_build_time</string>
```

## Dispatch

XML resource rule over `res/values*/strings.xml`. Reuses the
`StringsLocation` index and the existing resource parsing pipeline.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
