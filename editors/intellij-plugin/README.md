# Krit IntelliJ Plugin

Native JetBrains IDE bridge for Krit diagnostics.

This plugin keeps Krit as the source of truth. It shells out to the `krit`
binary, parses Krit JSON output, and maps findings to IntelliJ editor
annotations and inspections.

## Current Surfaces

- Live Kotlin, Java, and Android XML highlighting through `ExternalAnnotator`
- Inspect Code integration through `LocalInspectionTool`: one inspection per
  krit rule under `Settings → Inspections → Krit → <category>`, plus the
  per-language Krit inspections (short names `Krit`, `KritJava`, `KritXml`;
  see [Per-rule inspections](#per-rule-inspections))
- Severity mapping for `error`, `warning`, and `info`; findings with
  confidence below 0.5 render as weak warnings
- A project scan on open, then rescans of edited files after the scan debounce
  (`Settings → Tools → Krit → Scan debounce`, default 1000 ms)
- Native quick fix for fixable findings: `krit --fix --fix-level <level>
  --finding-id <id>`, offered when the fix's level is within
  `Settings → Tools → Krit → Maximum fix level`; rule suggestions are applied
  with `krit apply-suggestion`; a Suppress quick fix adds `@Suppress` /
  `@SuppressWarnings`
- `Krit: Analyze File` (editor and project view context menus) and `Krit: Analyze Project` (project view context menu), also under `Tools → Krit`; both scan immediately instead of waiting for the edit debounce
- Per-file rescans after an edit or a fix go through the project's krit daemon when one is running (`analyze-buffer` over its Unix socket), and through the CLI otherwise
- On project open the plugin starts a daemon (`krit daemon start --repo <project>`, which needs `krit-daemon` next to `krit`) when none is running, and stops it on close; a daemon you started yourself is left running. Turn this off with `Settings → Tools → Krit → Start a krit daemon for this project`
- Creating, editing, or deleting `krit.yml` / `.krit.yml` at the project root (or the configured config file) rescans the project

## Settings

`Settings → Tools → Krit` (application-wide) and `Settings → Tools → Krit →
Project` (per project):

| Setting | What it does |
|---|---|
| Krit binary | The krit executable. A project value overrides the application value (see the resolution order below). |
| Maximum fix level | The most invasive autofix the quick fixes apply: `cosmetic`, `idiomatic` (default, like krit's own `--fix-level`), or `semantic`. Autofixes above it are not offered; rule suggestions always are. |
| Config file | Passed to krit as `--config` instead of the project's `krit.yml` / `.krit.yml`. A relative path resolves against each project's root. The krit daemon only reads the project's own config, so while this is set the plugin runs scans and fixes through the CLI (`--no-daemon`) and does not start a daemon. Editing that file rescans the project. |
| Scan debounce (ms) | Delay after the last edit before a rescan (minimum 100). |
| Start a krit daemon for this project | See above. |

## Per-rule inspections

On project open the plugin reads the rule list of the project's krit binary
(`krit --list-rules --format json`) and adds one inspection per rule, named
after the rule, under `Settings → Inspections → Krit → <category>`. Their
enabled state, severity, and scopes are stored in the inspection profile like
any other inspection's, so they persist across restarts and can be shared
through a project profile.

The rule list is cached per krit binary (by its path, size, and modification
time) in memory and under the IDE system directory (`krit/rule-lists`), so
krit runs for it once per binary rather than on every project open. Replacing
the binary (an upgrade, a different path in settings) reads the list again.
`Tools → Krit → Krit: Refresh Rule List` drops the cached list and reads it
again; it adds inspections for new rules right away, while rules krit no
longer has leave the tree on the next IDE restart. A krit too old to print the
JSON list leaves the per-rule inspections out, and every finding is reported
by the per-language inspections as before.

### Precedence with krit.yml

- **krit.yml decides which rules run.** krit computes findings with its own
  config (`krit.yml`, `.krit.yml`, or the configured config file); the IDE
  never enables a rule krit.yml disables, and a per-rule inspection for such a
  rule simply has nothing to show.
- **The inspection profile filters what is shown.** Turning a rule's
  inspection off hides its findings in the editor and in Inspect Code (krit
  still computes them). Scopes work as usual.
- **Severity:** while a rule's inspection is at its default level (the rule's
  registry severity), findings show the severity krit reported, so severity
  overrides in krit.yml still apply. Once you change the level in the
  profile, the profile's level wins.
- The per-language inspections (`Krit`, `KritJava`, `KritXml`) are the master
  switch: with all three off, the plugin stops scanning. They also report
  findings of rules that have no per-rule inspection.

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

1. `Settings → Tools → Krit → Project → Krit binary` (this project only)
2. `Settings → Tools → Krit → Krit binary`
3. `-Dkrit.binary=/absolute/path/to/krit`
4. `KRIT_BINARY=/absolute/path/to/krit`
5. `krit` on `PATH`

A configured path that isn't executable is reported as a missing binary rather
than falling through to the next source.

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
