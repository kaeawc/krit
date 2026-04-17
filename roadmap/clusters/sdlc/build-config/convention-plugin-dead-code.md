# ConventionPluginDeadCode

**Cluster:** [sdlc/build-config](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Concept

Convention plugins defined in `build-logic/` (or `buildSrc/`) that
are never applied by any module.

## Triggers

`build-logic/src/main/kotlin/kotlin-library-conventions.gradle.kts`
is never referenced in any `plugins { id("...") }` block.

## Does not trigger

Every convention plugin is applied somewhere.

## Dispatch

`BuildGraph` walk + plugin-id registry of convention plugins.

## Links

- Parent: [`../README.md`](../README.md)
