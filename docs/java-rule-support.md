# Java Rule Support Inventory

This inventory tracks rule families for Java-only and mixed Java/Kotlin Android projects. Rules without an explicit `Languages` list still default to Kotlin, so Java support is counted only when the registry declares `scanner.LangJava` or the rule runs through a non-source pipeline such as XML, Gradle, manifests, or resources.

The machine-checked readiness gate lives in [`java-support-readiness.yml`](java-support-readiness.yml). It distinguishes infrastructure support, rule parity, Java autofix coverage, and accepted pending work so the Java Source Support milestone is not closed on an ambiguous definition of "full support."

## Supported Java Source Rules

| Category | Rules | Notes |
| --- | --- | --- |
| Android correctness | `DefaultLocale`, `CommitPrefEdits`, `CommitTransaction`, `CheckResult`, `SimpleDateFormat`, `WrongViewCast` | Java tests cover positive and local-lookalike negative cases where receiver identity matters. |
| Android security | `SetJavaScriptEnabled`, `AddJavascriptInterface`, `GrantAllUris`, `SecureRandom`, `WorldReadableFiles`, `WorldWriteableFiles`, `HandlerLeak` | Java tests cover Android API usage and local lookalikes. |
| Android source | `UseValueOf` | Java constructor and local-lookalike coverage. |
| Database/resource cost | `DatabaseInstanceRecreated`, `BufferedReadWithoutBuffer`, `CursorLoopWithColumnIndexInLoop`, `RecyclerAdapterWithoutDiffUtil`, `RecyclerAdapterStableIdsDefault`, `OkHttpClientCreatedPerCall`, `RetrofitCreateInHotPath`, `HttpClientNotReused`, `DatabaseQueryOnMainThread` | Java support uses Java AST node types plus source-level imports and type facts where available. |
| Security literal checks | `HardcodedBearerToken`, `HardcodedGcpServiceAccount` | Java support added in #700. These inspect `string_literal` nodes and avoid comments or arbitrary line scans. |
| Release engineering literal checks | `HardcodedLocalhostUrl` | Java support added in #700. Test/debug source paths remain excluded. |
| Exceptions | `ExceptionRaisedInUnexpectedLocation`, `ThrowingExceptionInMain` | Java support added in #700 for method-level throw checks that only need Java `method_declaration` and `throw_statement` nodes. |

## Supported Non-Source Android Rules

XML resource, Android manifest, Gradle, version-catalog, icon, and resource-value rules apply independently of whether app source is Java, Kotlin, or mixed. These rules are not Java source ports, but they are already useful in Java-only Android projects.

## Mixed Java/Kotlin Source Graph

The cross-file index now exposes a language-tagged source resolver for cheap mixed-source facts:

- Java files can resolve imported Kotlin classes and their source-visible callables.
- Kotlin files can resolve imported Java classes and Java members indexed from source.
- Java getter/setter calls such as `getDisplayName()`, `setEnabled(...)`, and `isActive()` add references to the corresponding Kotlin property names so module-aware and dead-code checks do not mark those properties unused.

Conservative gaps remain for Kotlin JVM shapes that require compiler lowering details: overloaded property accessors, `@JvmName`, `@JvmStatic`, `@JvmField`, file-facade naming overrides, companion-object bridge methods, and generated code from annotation processors. Those should stay source-visible where possible and use KAA/javac-backed facts only when the source index cannot answer precisely.

## Java Type Profile

Java-aware rules can use a shared checked-in type profile from `librarymodel.Facts.Java` for common Android, JDK, and dependency-gated library facts. The profile currently covers high-value framework types used by Java rules, known method returns such as `SharedPreferences.edit()`, `FragmentManager.beginTransaction()`, `WebView.getSettings()`, selected JDK string returns, and annotation aliases such as `androidx.annotation.CheckResult`.

Library-specific entries stay conservative: Room Java annotation facts are enabled when Room is present or dependency extraction is incomplete, and disabled when Gradle facts prove Room is absent.

## Java Autofix Support

Java-only source runs participate in text autofix when active rules declare Java support. The first low-risk Java fixes are cosmetic line edits shared with Kotlin: trailing whitespace removal, tab indentation replacement, and blank-line insertion after package/import declarations. The fixable fixture harness accepts `.java` before/after pairs, and CLI coverage verifies `krit --fix` mutates Java-only targets instead of exiting early when no Kotlin files are present.

## Pending Java-Applicable Batches

| Batch | Status | Plan |
| --- | --- | --- |
| Potential bugs | Pending | Port rules whose Java AST shapes are clear first. Mark rules needing receiver/return types for source facts or javac-backed facts instead of adding broad lexical heuristics. |
| Exceptions | Partial | Method-level throw rules are done. Continue with catch/finally rules using Java `try_statement`, `catch_clause`, `finally_clause`, and method invocation nodes. |
| Privacy | Pending | Port storage/logging/analytics rules that depend on shared Android APIs. Add local-lookalike negatives for API names such as `SharedPreferences`, `Log`, and analytics clients. |
| Release engineering | Partial | Literal URL support is done. Follow with Java-safe logging/import/build-config rules where AST support is adequate. |
| Security | Partial | Literal credential support is done. Follow with Java-safe source rules that can use imports/source facts or explicit future javac fact requirements. |
| Style and naming | Pending | Classify carefully; many style rules encode Kotlin syntax and should remain Kotlin-only unless a Java-specific implementation is designed. |
| Autofix | Partial | Low-risk cosmetic line fixes now support Java with `.java` fixable fixtures and CLI coverage. Semantic Java fixes still need separate safety review and Java fixtures. |

## Kotlin-Only or Needs Design

Rules tied to Kotlin-only syntax or semantics stay Kotlin-only until there is a Java-specific design. Examples include coroutine rules, Compose rules, Kotlin null-safety operators, data class rules, extension-function rules, companion/object rules, Kotlin collection idioms, ktfmt-oriented formatting rules, and FIR/KAA-only checks.

Rules that require resolved Java overloads, dependency annotations, exact receiver types, or inherited type facts should be marked for source facts or javac-backed facts before Java activation.
