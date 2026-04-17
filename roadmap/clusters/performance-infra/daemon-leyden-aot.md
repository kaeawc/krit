# Daemon Leyden AOT (Phase 4)

**Cluster:** [performance-infra](./README.md) · **Status:** planned ·
**Supersedes:** Phase 4 of [`roadmap/21-daemon-startup-optimization.md`](../../21-daemon-startup-optimization.md)

## What it is

Replace CRaC checkpoint/restore with Project Leyden AOT compilation for the
Kotlin Analysis API daemon (`tools/krit-types/`). Leyden produces a native-like
binary from the JVM app, eliminating cold-start overhead entirely.

## Current state

Phases 1-3 shipped: AppCDS archive, persistent daemon with PID-hashed routing,
CRaC checkpoint support. Daemon hardened with `analyzeWithDeps` protocol,
timeout/output-watchdog, JVM flag retune.

## Blocker

Requires JDK 25+ (Leyden is an OpenJDK project in early access). Not yet
available in production JDK releases.

## Implementation notes

- Primary file: `internal/oracle/daemon.go`
- When JDK 25 ships, replace CRaC with Leyden AOT compilation step
- Expected impact: cold start from ~2s (CRaC) to <500ms
