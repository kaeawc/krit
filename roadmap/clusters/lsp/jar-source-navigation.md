# JarSourceNavigation

**Cluster:** [lsp](README.md) · **Status:** open (optional) ·
**Severity:** n/a (infra) · **Default:** n/a · **Est.:** 3–5 days

## What it does

Enables goto-def into symbols that live in JARs on the classpath —
`kotlinx.coroutines.CoroutineScope`, `kotlin.collections.List`, any
third-party library. Produces synthetic `jar://` URIs that the LSP
client renders as read-only virtual documents, with a fabricated
Kotlin source derived from the JAR's bytecode via the Kotlin
Analysis API's decompiler.

Optional because milestones 1–5 already deliver within-project
navigation, which is 90% of the value. This milestone closes the
remaining gap: jumping into stdlib and library sources without
leaving the editor.

## Current cost

Today, goto-def on `CoroutineScope` from Android app code returns
nothing (single-file textual walker has no visibility). After
milestones 1–3, it returns nothing *correctly* — the oracle knows
the FQN resolves to `kotlinx.coroutines.CoroutineScope` but has no
source location because the declaration lives in a JAR we don't have
sources for.

Workaround users accept today: Cmd+click fails, they switch to
Android Studio / IntelliJ for navigation, come back to Cursor / VS
Code for everything else. Friction, not blocker.

## Proposed design

### Decompile path

Kotlin Analysis API provides `KotlinDecompiledLightClassSupport` and
friends that render JAR class files (with or without Kotlin metadata)
as readable Kotlin source. Used by IntelliJ's "Decompiled File"
feature. We plug this into `krit-types`:

```
/jar/decompile?jar=/path/to/kotlinx-coroutines-core.jar&fqn=kotlinx.coroutines.CoroutineScope
→ Kotlin source text + line-level offset map for the requested symbol
```

Cached on disk at `.krit/jar-decompile/{jar-sha}/{fqn}.kt`. JAR
contents are immutable per content hash, so cache invalidation is
never needed.

### Synthetic URIs

The LSP spec allows servers to return `Location` entries with any
URI scheme. We mint `krit-jar://` URIs:

```
krit-jar:///kotlinx-coroutines-core/3.7.1/kotlinx/coroutines/CoroutineScope.kt
```

The LSP server handles these via a new `textDocument/` document
provider — client sends `textDocument/didOpen` for the URI, server
responds with the decompiled source. Most LSP clients handle read-only
virtual documents natively; VS Code requires a client-side
`vscode.workspace.registerTextDocumentContentProvider` hook in the
extension, which is ~20 lines of TypeScript.

### Integration with navigation handlers

When `handleDefinition` or `handleReferences` finds a declaration
whose `File` starts with a JAR path, it:

1. Extracts `{jar-path, fqn}`.
2. Asks the oracle for a decompile via the daemon RPC.
3. Writes the decompile to the on-disk cache if not already there.
4. Returns a `Location` with a `krit-jar://` URI.

Client opens the virtual document, renders it read-only.

### Classpath discovery

Finding the right JAR for a given FQN requires knowing the project's
classpath. Gradle/Maven projects have this in their build metadata.
`krit-types` already passes a `--classpath` flag for resolution
precision (mentioned in `oracle-optimization-session-summary.md`).
This milestone depends on that flag being populated, either from:

- An explicit `krit.yml` `classpath:` setting, or
- Auto-detection from `.gradle/` / `.kotlin/` caches, or
- A fallback to `JAVA_HOME/jre/lib/rt.jar` for stdlib-only decompile.

Partial resolution is fine: if we can't find the JAR, return the
signature-stub `Location` pointing at the krit-jar URI anyway with
a placeholder body, and log the unresolved classpath entry.

### Line/position accuracy

The decompiler regenerates source from bytecode — line numbers won't
match the original repo sources. The oracle's `DeclLocation.Line`
inside a decompiled file is the regenerated line, not the upstream
line. That's fine; users don't know the upstream's line numbers and
don't care, as long as the decompiled file places the declaration
consistently.

## Files to touch

- `krit-types` (JVM side) — new `decompileJar(jar, fqn)` RPC method
- `internal/oracle/decompile.go` — Go-side cache + daemon call
- `internal/lsp/jar_uri.go` — URI scheme construction and parsing
- `internal/lsp/server.go` — `textDocument/didOpen` handler for
  `krit-jar://` URIs
- `editors/vscode/src/extension.ts` — register
  `vscode.workspace.registerTextDocumentContentProvider` for
  `krit-jar://`

## Testing

- Goto-def on `CoroutineScope` → returns `krit-jar://` URI →
  client opens it → decompiled source shows the interface
  declaration.
- Goto-def on `Unit` → stdlib decompile (classpath has
  `kotlin-stdlib.jar`).
- Find References for a JAR symbol: the symbol's declaration site
  is in the JAR (decompiled), call sites are in the workspace.
  Mixed result list renders both.
- Cached decompile: first access is slow (seconds), second is
  instant.
- Classpath missing: logged warning, placeholder Location returned,
  handler doesn't crash.

## Risks

- **Decompile quality is best-effort.** Java-sourced libraries
  render as Kotlin with artificial naming. Users expecting original
  sources will be mildly disappointed. Document the limitation.
- **Classpath discovery is messy.** Gradle, Maven, Buck, Bazel all
  record the classpath differently. Start with Gradle-only auto-detect;
  manual `classpath:` in `krit.yml` for others.
- **JAR license concerns**. Some JARs have licenses restricting
  decompilation or redistribution. The decompile is strictly
  local-use and never leaves the user's machine; document this
  explicitly.
- **URI scheme friction**. Not all LSP clients render virtual
  documents cleanly. Neovim needs some setup; IntelliJ LSP4IJ has
  read-only virtual-document support; VS Code works with the content
  provider hook. Document editor-specific steps.

## Blocking

- Nothing — this is the optional polish milestone.

## Blocked by

- [`navigation-handlers-rewrite.md`](navigation-handlers-rewrite.md)
  (milestone 3) — needs the handler path that delegates to the
  oracle before we can intercept JAR-resolved declarations.
- [`didchange-oracle-refresh.md`](didchange-oracle-refresh.md)
  (milestone 4) recommended for daemon mode; one-shot decompiles
  via cold `krit-types` launches work but are slow per-request.

## Links

- Parent cluster: [`lsp/README.md`](README.md)
- Related: Kotlin Analysis API `KotlinDecompiledLightClassSupport`
  (IntelliJ sources)
- Classpath precision note: "JDK classpath for resolveToCall" in
  `oracle-optimization-session-summary.md` (if retained) —
  ~30-minute fix to auto-detect `$JAVA_HOME` classpath, which is
  a prerequisite for good FQN resolution even within the workspace
