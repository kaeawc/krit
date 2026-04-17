# Gradle Plugin

**Cluster:** [build-integration](./README.md) · **Status:** in-progress ·
**Supersedes:** [`roadmap/01-gradle-plugin.md`](../../01-gradle-plugin.md)

## What it is

Gradle plugin (`krit-gradle-plugin/`) that integrates krit into Gradle builds.
Downloads or locates the native Go binary and invokes it as an external process
(unlike detekt which runs on the JVM via classloading).

## Current state

Phase 1 complete (scaffolding): KritPlugin, KritExtension, KritCheckTask,
KritFormatTask, KritBaselineTask, KritBinaryResolver, KritReports, unit tests.
All in `krit-gradle-plugin/`.

## Remaining work

- **Phase 3–5:** Performance optimization (incremental analysis, up-to-date
  checks), KMP support (expect/actual source sets), polish (error messages,
  documentation, Gradle portal publishing)
- Binary resolver needs checksum verification (see `sdlc/release-signing.md`)
