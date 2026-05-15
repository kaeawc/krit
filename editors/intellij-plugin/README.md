# Krit IntelliJ Plugin

Native JetBrains IDE bridge for Krit diagnostics.

This plugin keeps Krit as the source of truth. It shells out to the `krit`
binary, parses Krit JSON output, and maps findings to IntelliJ editor
annotations and inspections.

## Current Surfaces

- Live Kotlin highlighting through `ExternalAnnotator`
- Inspect Code integration through `LocalInspectionTool`
- Severity mapping for `error`, `warning`, and `info`
- Project-wide background Krit runs every 5 seconds, skipping ticks while a prior run is active
- Native quick fix action for fixable findings; it invokes `krit --fix --fix-level idiomatic`

## Local Development

```bash
../../tools/krit-fir/gradlew test
../../tools/krit-fir/gradlew runIde
```

The plugin resolves the Krit binary in this order:

1. `-Dkrit.binary=/absolute/path/to/krit`
2. `KRIT_BINARY=/absolute/path/to/krit`
3. `krit` on `PATH`

## Intentional Limits

The plugin does not implement rules. It only translates Krit findings into
JetBrains IDE diagnostics.
