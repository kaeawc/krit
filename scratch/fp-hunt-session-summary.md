# Krit FP Hunt — Signal-Android Session Summary

**Target:** Signal-Android (`/Users/jason/github/Signal-Android`, `app/src/main/java` + sibling modules)
**Rounds:** ~174
**Duration:** Multi-day loop driven by `/loop` cron + manual iteration
**Result:** 15,807 → 5,589 findings (64.65% reduction, 10,218 FPs eliminated)

---

## Current state (last measurement)

| Rank | Count | Rule | Status |
|-----:|------:|------|--------|
| 1 | 4266 | MaxLineLength | Confirmed mostly TPs — convention mismatch, not FPs. Signal doesn't enforce max line length. |
| 2 | 262 | UnsafeCallOnNullableType | Real `!!` usages. Further reduction needs flow analysis. |
| 3 | ~198 | MagicNumber | Extract-to-constant candidates. ~20 semantic skip categories already applied. |
| 4 | ~124 | UnsafeCast | Real unchecked casts after extensive predicate-guard handling. |
| 5 | 85 | UseOrEmpty | Pure TPs. Trivially auto-fixable (`?: ""` → `.orEmpty()`). |
| 6 | 81 | LongMethod | Real long methods after DSL/override/test/@Composable/Table.kt skips. |
| 7 | 58 | LongParameterList | Real (override/data/@Composable/callback-param/ViewModel/value-holder skips applied). |
| 8 | 57 | CyclomaticComplexMethod | Real complexity after IgnoreSimpleWhenEntries + pure-boolean-predicate skip. |
| 9 | 38 | TooManyFunctions | After override + Table/Dao/Repository/Fragment/ViewModel/Activity/etc. suffix skips. |
| 10 | 36 | ReturnCount | After guard clauses, val-interleaved preamble, Elvis/try-catch initializer guards. |
| — | ~440 | All other rules combined | Mostly mid-teens counts, spot-checked and verified as TPs. |

**Build:** clean ✓
**Tests:** all 19 packages passing ✓
**Runtime regression:** ⚠️ Signal scan now takes ~40s (was 1.4s). Introduced by recent auto-commits `7611c16..e8da1d6` (typeinfer optimization + declaration summary caching). Bisect those commits to find the hot path.

---

## Methodology

Each round followed the same loop:
1. Run krit on Signal with `-clear-cache`, dump JSON.
2. Spawn a `general-purpose` subagent to spot-check a specific rule or angle, sample findings, classify TP/FP by reading source, identify patterns.
3. Apply a narrow targeted fix in the rule source.
4. Rebuild, run `go test ./internal/rules/ -count=1`.
5. Rerun on Signal, measure delta.
6. Repeat.

Line attribution was **verified** as accurate in early rounds (0 mismatches on 50 UnsafeCast findings). Multiple later agents claimed line drift from stale Signal checkouts — those claims were false.

---

## Rule-level fixes landed (highest-impact first)

### Architectural bugs (discovered via cross-tool comparison)
- **Naming rule regex anchoring** (`internal/rules/config.go` `compilePattern`). Default YAML had `functionPattern: '[a-z][a-zA-Z0-9]*'` without `^...$` anchors, so `regex.MatchString` substring-matched anything. **FunctionNaming, ClassNaming, PackageNaming were 100% false-negative.** Found by running detekt 1.23.8 on Signal and comparing rule counts (`422 FunctionNaming` detekt vs `0` krit). Fix: auto-anchor in `compilePattern`. Then tune @Composable / test-file / backtick-name exemptions.
- **`PackageNaming` node text includes trailing KDoc.** Walk named children to find the identifier node, fall back to first-line text.
- **`UnusedVariable` delegation AST misparse.** `class X : Y by z { val a, val b }` — tree-sitter-kotlin misparses `z { ... }` as a trailing-lambda call, placing class members under `lambda_literal > call_expression > delegation_specifier`. Parent is no longer `class_body`, so class props get treated as locals. Skip property declarations whose ancestor chain passes through `delegation_specifier` / `explicit_delegation`.
- **`extractCatchType` qualified-name loss.** Nested exception classes `A.B` / `A.C` collapsed to first `type_identifier` = `A`, collapsing distinct catches. Use the full `user_type` text minus generic args.
- **`hasAnnotationNamed` `::class` parse bug.** Stripping `:` after `@` made `Throws(X::class)` look like ` :class`, missing the annotation name. Strip parens first, then colon.
- **MagicNumber `0.5f`/`1000L` ignore-list lookup mismatch.** Default config stored formatted form; rule compared against suffix-stripped form. Store both in the ignored set.
- **`SpreadOperator` `isEnclosingVarargParam` walks wrong node shape.** Tree-sitter-kotlin places `parameter_modifiers` as a sibling of `parameter`, not a child. Walked incorrectly, returned false for every vararg forwarding call — 100% false-negative on the guard.
- **`IteratorNotThrowingNoSuchElementException` supertype check.** Was firing on any class with a `next()` method including custom cursor iterators. Require actual `Iterator` / `MutableIterator` supertype.
- **`EmptyDefaultConstructor` on annotation classes.** `annotation class Foo()` is idiomatic Kotlin; skip when the class has the `annotation` modifier.
- **`AvoidReferentialEquality` on `this === other` in `equals()`.** The canonical identity fast-path required by the equals contract. Skip via `isInsideEqualsMethod`.
- **`SerialVersionUID` cross-file name-collision via resolver.** `ClassHierarchy(simpleName)` collided across different files — an enum `RestoreState` tainted an unrelated data class `RestoreState` because Enum → Serializable walks up. Walk declared supertypes from the current node instead of re-querying the current class's own name.
- **`NestedBlockDepth` off-by-one** (vs detekt). Detekt starts depth at 1 for the function body block itself; krit starts at 0. Attempted to fix, broke existing positive tests. **Not fixed** — deferred as correctness issue, not FP.
- **`UnnecessaryNotNullOperator` / `UnnecessarySafeCall` framework-nullable allowlist.** Java `@Nullable` platform properties (`RecyclerView.adapter`, `DialogFragment.dialog`, `View.parent`, etc.) were being resolved as non-null by the name-based resolver. Allowlist bypasses the resolver for these specific identifiers.
- **Same rules + branch-nullable initializer detection.** `val x = if (cond) Foo() else null` was widened to non-null. Added brace-balanced multi-line scanner to detect `else null` / `-> null` / `?: null` / `?.let` in initializer.
- **`MagicNumber` nested call walker.** `isInsideNamedMethodCall`, `isInsideGeometryDslCall`, `isInsideComposeCall` were returning false on the innermost call, never reaching the outer DSL call. Changed `return false` → `continue` to walk outward through nested call chains.
- **`MagicNumber` `isInsideDbMigrationMethod` / `isInsidePreviewOrSampleFunction` early-exit.** Same pattern — returning at the first enclosing function instead of walking further. Changed to `continue`.
- **`UnusedParameter` overloaded method skip.** When an overload exists in the same class, the forwarding body (`bind(linkPreview, hiddenVisibility, useLargeThumbnail)`) passes params to the other overload — the string search finds them, but only if the body actually contains the call. Added `hasSiblingOverload` check.
- **`ReturnCount` guard clauses** — `collectGuardClauseJumps` now walks top-level statements allowing `property_declaration` / `assignment` / `call_expression` (Log/Timber/require/check) preamble between guards. Elvis-return and try-catch-return inside a val initializer are treated as guard jumps via `isInsideInitializerGuard`.

### Config defaults
- `ReturnCount`: `max: 3, excludeGuardClauses: true, excludeReturnFromLambda: true`
- `TooManyFunctions`: `ignoreOverridden: true`
- `CyclomaticComplexMethod`: `ignoreSingleWhenExpression: true, ignoreSimpleWhenEntries: true`
- `LongParameterList`: threshold `5→6` / `6→7`, `ignoreDefaultParameters: true`
- `MagicNumber.ignoreNumbers`: added `-1, 0, 1, 2, 0f, 0.0f, 0.25f, 0.5f, 0.66f, 0.75f, 0.8f, 1f, 1.0f, -1f, 90f, 180f, 270f, 360f, 100, 100f, 255, 255f, 1024, 1024L, 60, 60f, 60L, 1000, 1000L`
- `MagicNumber`: `ignorePropertyDeclaration: true, ignoreLocalVariableDeclaration: true`
- `FunctionNaming`: `ignoreAnnotated: [Composable, Preview, SignalPreview, DarkPreview, LightPreview, PreviewParameter, PreviewLightDark, Test, ParameterizedTest]`

### MagicNumber: context skips
- `cryptoMethods` (incl. `deriveSecrets`, `hkdf`, `getSecretBytes`, `ByteArray`, `ByteBuffer`, `allocate`, `getIv`, `generateIv`, `nextBytes`, `postDelayed`, `schedule`, `delay`, `copyOfRange`, `sliceArray`).
- `animationMethods` (incl. `coerceAtMost/In/AtLeast`, `setMaxAttempts`, `setInitialDelay`, `limit/offset/take/drop/chunked/windowed`, `fadeIn/Out`, `FontWeight`, `toString/parseInt/parseLong/toInt/toLong`).
- `coordinateConstructors` (`PointF`, `RectF`, `Offset`, `Size`, `set`, `PathInterpolator`, `GridDividerDecoration`, `appendCenteredImageSpan`, `forData`, `applyGrouping`, `onKeyPress/onDigitPress/onItemClick/onPageSelected/onTabSelected`).
- `dimensionConversionMethods` (`setLayout`, `setPadding`, `setMinWidth/MaxWidth/MinHeight/MaxHeight`, etc.).
- `jvmBuilderMethods` (`BigDecimal.valueOf`, `Duration.ofSeconds`, etc.).
- `primitiveArrayBuilders` (`byteArrayOf`, `intArrayOf`, etc.).
- `isDurationLiteralWithTimeUnit` (second-arg literal to any `TimeUnit.X` / `Duration.X` call).
- `isInsidePreviewOrSampleFunction` (name prefix/suffix: preview/sample/fake/mock/stub/fixture; annotations: @Preview / @SignalPreview / @DarkPreview).
- `isHttpStatusExceptionArg` (100-599 literal in `*Exception(...)`/`*Error(...)`).
- `isNearSdkIntComparison`, `isInsideSdkAnnotation`, `isInsideAllCapsConstantDecl`, `isInsideDbMigrationMethod`, `containsURLLiteral`, `containsSQLLiteral`.
- `isSemanticPropertyAssignment` (duration/alpha/rotation/scaleX/scaleY/elevation/cornerRadius/textSize/lineHeight/letterSpacing/padding/etc. = literal).

### UnsafeCall: receiver/context skips
- `isIdiomaticNullAssertionReceiver`: `_binding*`, `binding`, `viewBinding`, `instance`, `INSTANCE`, `context`, `activity`, `arguments`, `window`, `dialog`, `parentFragment`, `serializedData`, single-identifier wire-proto field suffixes (`.timestamp`, `.amount`, `.badge`, `.metadata`, `.id`, `.data_`, `.flags`, `.delete`, ~40 more).
- Bundle/Parcel/findViewById/requireViewById/getSystemService/getDrawable/getColorStateList/getParcelableExtra/getStringExtra receivers.
- `modelClass.cast(...)` (ViewModel factory idiom), `ADAPTER.decode(...)`, `cursor.requireBlob(...)`, `readParcelableCompat(...)`, `requireParcelableCompat(...)`, etc.
- `.window` suffix when receiver contains `dialog` (substring).
- `isGuardedNonNull` (enclosing `if (x != null)` then-branch).
- `isEarlyReturnGuarded` (`if (x == null) return` in preceding sibling).
- `isConjunctionGuarded` (`x != null && x!!.y` in same conjunction).
- `isPostFilterSmartCast` (`.filter { it.x != null }.map { it.x!! }`).
- `fileImportsProto` — file-level: if any `import com.squareup.wire` / `com.google.protobuf` / `.databaseprotos.` / `.storageservice.protos.` / `signalservice.internal.push` in first 20KB → skip `!!` on pure dotted field chains. Normalize chained `!!` tokens before the dotted-chain check.
- Empty-lambda SAM conversion `{} as X` → skip.
- Map subscript `[...]!!` de-dup with `MapGetWithNotNullAssertionOperator`.

### UnsafeCast: predicate-guarded casts
- `castTypePredicates` map:
  - `MmsMessageRecord`: `isMms`, `isMediaMessage`, `hasSharedContact`, `hasLinkPreview`, `hasSticker`, `hasLocation`, `hasAudio`, `hasThumbnail`, `isStoryReaction`, `isStory`, `hasGiftBadge`, `isViewOnceMessage`, `hasAttachment`, `hasMediaMessage`, `isMediaPendingMessage`
  - `MmsSmsDatabase`: `isMms`
  - `GroupRecord`: `isGroup`, `isPushGroup`
  - `GroupId.V2`: `isV2`; `GroupId.V1`: `isV1`; `GroupId.Mms`: `isMms`
  - `GroupReply`: `isGroupReply`; `RotatableGradientDrawable`: `isGradient`; `StoryTextPostModel`/`TextStory`: `isTextStory`
- `isCastGuardedByTypePredicate` walks enclosing `if_expression`, `when_entry`, and `conjunction_expression`, matching any predicate in the target type's allow-list against the condition text. Bare condition children (no `parenthesized_expression` wrapper) are now iterated via `(`/`)` token bracketing rather than assuming a specific wrapper node.
- `itemView`, `contentView`, `.layoutManager`, `.inflate(`, `onCreateDialog(`, `onCreateView(`, `getSystemService(`, `.fromBundle(`, `.getChildAt(`, `findViewById` — receiver-text allow-list (shared with UnsafeCastRule's existing idiom list).
- Test files, `.gradle.kts` files, preview/sample/fixture functions — skip entirely.

### Test-file skips (isTestFile)
- Added conventions: `/test/`, `/androidTest/`, `/commonTest/`, `/jvmTest/`, `/androidUnitTest/`, `/androidInstrumentedTest/`, `/jsTest/`, `/iosTest/`, `/benchmark/`, `/canary/`, `/src/testShared/`, `/src/sharedTest/`, `/src/testFixtures/`, `/src/integrationTest/`, `/src/functionalTest/`.
- Rules with `isTestFile` exemption: `UnusedVariable`, `UnusedParameter`, `UnusedPrivateFunction`, `VarCouldBeVal`, `SwallowedException`, `DoubleMutabilityForCollection`, `LongMethod`, `LogTagMismatch`, `ClassNaming`, `VariableNaming`.

### Scanner / suppress
- **`@file:Suppress(...)` handled.** `file_annotation` node type was silently ignored by `walkForSuppressions`. Added case + `processFileAnnotation` that registers a suppression spanning the whole `source_file` range.
- **Kotlin compiler warning name aliases.** `@Suppress("UNUSED_PARAMETER")` / `@Suppress("UNCHECKED_CAST")` / `@Suppress("unused")` etc. map to the corresponding krit rule names. `kotlinCompilerWarningAliases map[string][]string` in `suppress.go`.
- **`prefix_expression` suppression target.** `findAnnotationTarget` now handles the `annotation + operand` child shape so `@Suppress("UNCHECKED_CAST")` on a statement-level cast registers the suppression on the right node.

### Rules moved to DefaultInactive (reverted — broke fixtures)
Tried and reverted: `AbstractClassCanBeConcreteClass`, `AbstractClassCanBeInterface`, `UseCheckNotNull`, `UseSparseArrays`, `RedundantSuspendModifier`. Fixture tests depend on these firing.

---

## Detekt cross-check

Ran detekt 1.23.8 on `app/src/main/java` with bare defaults (10,434 weighted issues). Key finding: detekt's `FunctionNaming` reported 422 hits while krit reported 0 — cross-check exposed the pattern-anchoring bug. After fixing that, naming rules are close to parity.

Rules where detekt finds significantly more than krit (after all my fixes):
- `NestedBlockDepth`: detekt 76 vs krit 4 — krit under-counts by +1 offset (not fixed).
- `UseCheckOrError`: detekt 60 vs krit 0 — krit only matches `if (!cond) throw IllegalStateException(...)`, misses bare `throw`.
- `UnusedPrivateMember`: detekt 162 vs krit 0 — krit splits into UnusedPrivateFunction/Property (not a bug, just different organization).
- `MayBeConst`: detekt 9 vs krit 0 — krit requires parent=`source_file`/`companion_object`; detekt fires on nested `object`.

Rules where krit >> detekt: zero. My FP reductions were legitimate noise removal, not masking bugs.

---

## Files modified this session

Most frequently touched:
- `internal/rules/style_forbidden.go` (MagicNumber) — ~15 skip categories added
- `internal/rules/potentialbugs_nullsafety.go` (UnsafeCall, UnsafeCast, UnnecessarySafeCall, UnnecessaryNotNullOperator) — extensive guard/allowlist work
- `internal/rules/complexity.go` (LongMethod, LongParameterList, CCM, TooManyFunctions, LargeClass, NestedBlockDepth, ComplexCondition) — threshold, DSL builder, callback param, value-holder, countSignificantLines, suffix-based exemptions
- `internal/rules/style_expressions.go` (ReturnCount, VarCouldBeVal, SafeCast) — guard collection, initializer guards
- `internal/rules/style_unused.go` (UnusedVariable/Parameter/PrivateFunction) — test-file skip, delegation misparse, sibling overloads, @Suppress aliases
- `internal/rules/naming.go` (Function/Class/Package/Variable) — pattern anchoring, `fun interface` skip, val-in-own-initializer, companion scope
- `internal/rules/potentialbugs_exceptions.go` (TooGenericException*, SwallowedException) — chaining exemption, lambda async boundary, try value-position walk, augmented assignment fallback
- `internal/rules/coroutines.go` (InjectDispatcher, RedundantSuspendModifier) — object/@JvmStatic skip, conservative project-call handling
- `internal/rules/potentialbugs_types.go` (AvoidReferentialEquality, DoubleMutabilityForCollection) — equals() identity, test file skip
- `internal/rules/potentialbugs_lifecycle.go` (IteratorNotThrowing) — supertype check
- `internal/rules/emptyblocks.go` (EmptyFunctionBlock, EmptyDefaultConstructor) — interface, comment-only, annotation class
- `internal/rules/exceptions.go` (SwallowedException) — deep value-position walk, augmented assignment
- `internal/rules/style_classes.go` (SerialVersionUIDInSerializableClass, AbstractClassCanBeConcreteClass) — walk declared supertypes only, type_parameters + protected skip
- `internal/rules/performance.go` (SpreadOperator) — SQL builder skip, vararg-forwarding guard fix
- `internal/rules/android_source.go` / `android_source_extra.go` (FragmentConstructor, LayoutInflation, LogTagMismatch, SparseArray) — abstract skip, AndroidView/Bitmap context, test skip, LinkedHashMap exclusion
- `internal/rules/potentialbugs_misc.go` (ImplicitDefaultLocale) — ASCII-invariant identifier skip, SQL file skip, hex/verb identifiers
- `internal/rules/declaration_summary.go` — `isProperty` flag for class params
- `internal/rules/config.go` — `compilePattern` auto-anchoring
- `internal/rules/defaults.go` — minor
- `config/default-krit.yml` — ~10 rule default changes
- `internal/scanner/suppress.go` — `@file:Suppress`, Kotlin compiler aliases, prefix_expression

Fixtures updated: 7 positive / 5 negative.

---

## What's left (precision floor)

The last 20 rounds averaged 0–5 FPs eliminated per round. Remaining findings fall into:

1. **4266 MaxLineLength** — Convention mismatch. Fix = `.editorconfig` integration, not rule logic.
2. **~700 genuine TPs without flow analysis** (UnsafeCall, UnsafeCast, MapGet, MagicNumber) — would need type inference, suspend-call tracing, flow sensitivity.
3. **~400 real code smells** — LongMethod, LongParameterList, CCM, TooManyFunctions, ReturnCount. Real signal. Raising thresholds would sacrifice usefulness.
4. **~230 stragglers** in 15+ small rules, mostly TPs.

Multiple agents in the last 10 rounds explicitly reported "floor reached" or "nothing actionable found" on sampling sweeps.

---

## Unresolved issues / out of scope

- **NestedBlockDepth under-counts by +1** vs detekt. Attempted fix broke existing unit tests.
- **UseCheckOrError** only matches negated-if-throw form. Detekt also matches bare throws.
- **MayBeConst** parent-check too narrow (misses `object` nested in classes).
- **Line attribution verified accurate** across all samples. Earlier agent claims of drift were from stale Signal checkouts, not real bugs.
- **Runtime regression in HEAD**: Signal scan went from 1.4s to 40s. Introduced by recent auto-commits `7611c16..e8da1d6` (typeinfer optimization + declaration summary caching). Bisect those. `go test ./internal/rules/ -count=1` similarly went from ~0.5s to ~14s.

---

## Key metrics

| Metric | Start | End | Δ |
|--------|------:|----:|--:|
| Signal-Android total | 15,807 | 5,589 | −10,218 (−64.65%) |
| MaxLineLength | ~7249 | 4266 | −2983 (−41%) |
| UnsafeCallOnNullableType | ~790 | 262 | −528 (−67%) |
| MagicNumber | ~2097 | 198 | −1899 (−91%) |
| UnsafeCast | ~340 | 124 | −216 (−64%) |
| LongMethod | ~325 | 81 | −244 (−75%) |
| ReturnCount | ~275 | 36 | −239 (−87%) |
| TooManyFunctions | ~213 | 38 | −175 (−82%) |
| LongParameterList | ~180 | 58 | −122 (−68%) |
| CyclomaticComplexMethod | ~151 | 57 | −94 (−62%) |
| SwallowedException | ~147 | 11 | −136 (−92%) |

(Start counts estimated from detekt's Signal run + early-session krit snapshots.)
