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

From the repository root, the AutoMobile-style helper scripts are:

```bash
scripts/ide-plugin/validate.sh
scripts/ide-plugin/install_from_source.sh
scripts/ide-plugin/watch_install.sh
```

`install_from_source.sh` builds the plugin zip, installs it into the detected
IntelliJ IDEA or Android Studio plugins directory, and restarts the IDE unless
`--no-restart` is passed. Set `KRIT_IDEA_PLUGINS_DIR`, `IDEA_PLUGINS_DIR`, or
`ANDROID_STUDIO_PLUGINS_DIR` when auto-detection picks the wrong IDE.

`watch_install.sh` rebuilds and reinstalls on plugin source changes. It is a
restart-based loop for extension-point changes, not IntelliJ dynamic plugin
reload.

The plugin resolves the Krit binary in this order:

1. `Settings → Tools → Krit → Krit binary` (when set and executable)
2. `-Dkrit.binary=/absolute/path/to/krit`
3. `KRIT_BINARY=/absolute/path/to/krit`
4. `krit` on `PATH`

## Publishing to JetBrains Marketplace

A tag matching `intellij-plugin-vX.Y.Z` triggers
`.github/workflows/publish-intellij-plugin.yml`, which builds the plugin
zip and uploads it to the Marketplace under the `dev.jasonpearson.krit`
id. The workflow needs a `JETBRAINS_MARKETPLACE_TOKEN` repository secret
(generated from the publisher account at
<https://plugins.jetbrains.com/author/me/tokens>).

The same workflow has a `workflow_dispatch` entry that takes a
`channel` argument (`default`, `beta`, or `eap`) and an optional
`version` override, so a maintainer can ship an unscheduled build
without cutting a tag.

## Intentional Limits

The plugin does not implement rules. It only translates Krit findings into
JetBrains IDE diagnostics.
