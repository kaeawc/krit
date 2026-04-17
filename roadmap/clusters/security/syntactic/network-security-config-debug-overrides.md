# NetworkSecurityConfigDebugOverrides

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`network_security_config.xml` with a `<debug-overrides>` element in
a release build variant, or a single shared file that contains the
override without a variant override.

## Example — triggers

```xml
<network-security-config>
    <base-config cleartextTrafficPermitted="false" />
    <debug-overrides>
        <trust-anchors>
            <certificates src="user" />
        </trust-anchors>
    </debug-overrides>
</network-security-config>
```

Flagged when the file lives in `src/main/res/xml/` (shared by all
variants) rather than `src/debug/res/xml/`.

## Example — does not trigger

```xml
<!-- in src/debug/res/xml/network_security_config.xml -->
<network-security-config>
    <debug-overrides>
        <trust-anchors>
            <certificates src="user" />
        </trust-anchors>
    </debug-overrides>
</network-security-config>
```

## Implementation notes

- New manifest/resource rule. Reuses the XML parsing pipeline in
  `internal/rules/android_resource_*.go`.
- Path check against the `src/debug/` segment.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: existing `InsecureBaseConfigurationManifest`
