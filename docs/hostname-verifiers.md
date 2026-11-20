# Hostname verifier rules

Both rules are enabled by default, but cover different hazards:

| Rule | Example |
| --- | --- |
| `BadHostnameVerifier` (`android-lint`) | A `javax.net.ssl.HostnameVerifier` with `override fun verify(host: String, session: SSLSession) = true`, or `HostnameVerifier { _, _ -> true }` |
| `AllowAllHostnameVerifier` (`security`) | `org.apache.http.conn.ssl.AllowAllHostnameVerifier()` or `org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER` |

Custom implementations no longer receive both findings. `BadHostnameVerifier` has Go and FIR
implementations. In FIR mode the compiler verdict replaces Go's for compiled Kotlin files;
degraded-mode fallback includes this rule. Go's original class-wide Android Lint visitor, signature matching,
same-file inheritance resolution and `verify` name anchor are retained. The visitor's last return
decides whether the class is complex: logging, declarations, throws and even an earlier `return false`
can be overridden by a final `return true`. Its class-wide overload quirks are retained too; this is
compatibility with the existing rule, not a proof that every execution returns true.

Additional source-proven literal-true methods on direct JDK verifiers and explicit
`HostnameVerifier { _, _ -> true }` SAM constructors retain safe cases previously reported only by
the old `AllowAllHostnameVerifier`. They emit at most one finding per class or SAM constructor.
The additional method check stops at nested owners, functions (including anonymous functions),
initializers, constructors and accessors. Enum classes and their entry overrides are also checked.
Unresolved SAM lookalikes are silent. Go does not infer implicit SAM conversions from setter names
alone; FIR checks the resolved JDK interface and the compiler's SAM conversion.

FIR retains the former custom-verifier checker's declaration and SAM coverage. Only return
expressions targeting the actual verifier member or SAM lambda contribute to its return-value check;
nested class bodies are skipped.
For every class, object, enum, enum entry, companion and SAM lambda, FIR reports only when every
targeted return yields the literal `true`; expression-bodied `if`/`when` branches must all yield that
literal as well. An earlier false/nonliteral return, a throwing path, or Kotlin `require`/`check`
guard makes the verifier silent. This is intentionally stricter than Go's Android Lint-compatible
last-return-wins behavior: for example, Go may report `if (...) return false; return true` while FIR
does not. Those Go-only findings are pinned in goldens rather than changing Go. Go's SAM check is
narrower still: it requires a single true statement. FIR does not reproduce Go's class-wide helper
overload false positives: only the resolved `verify(String, SSLSession)` member is considered.

## Main coverage self-check

The FIR port covers the resolved verifier and SAM shapes shown below, while deliberately dropping
Go/Lint findings whose return-order or throw behavior does not establish an always-true verifier.
Source-only Go coverage remains limited for compiler-resolved cases, as the table records. The FIR
goldens retain their diagnostic markers under `BadHostnameVerifier*` names; `// go-lines:` records
current Go findings, including intentional Go-only compatibility quirks. Go reports the `verify`
name; FIR reports the declaration's first annotation/modifier or `fun`, and SAM findings use the
call/lambda source.

| Kotlin shape from main | Current Go | Current FIR | Evidence (`BadHostnameVerifier*.kt`) |
| --- | --- | --- | --- |
| Direct class; expression `= true`; block `return true`; semicolon/comments | Yes | Yes | base golden |
| Nullable/FQN parameters; annotations/KDoc; multiline signature/body; `final`, abstract class, interface default | Yes | Yes | base golden |
| Enum member and enum entry override returning literal true on every path | Yes | Yes | base golden |
| Nested, inner and local class; anonymous object at top level, in a function or a factory | Yes | Yes | base golden, `GoMisses` |
| Named object or verifier companion, including a class and companion that both verify | No for object/companion | Yes | `GoMisses` |
| Inherited verifier base, qualified/nested base, long inheritance chain or cross-file base | Limited same-file lookup | Yes | `GoMisses`, `Inheritance`, `CrossFile` |
| Import aliases; direct/chained supertype or parameter type aliases | Parameter aliases and `kotlin.String`; limited supertype aliases | Yes | `ImportAlias`, `TypeAlias`, `GoMisses` |
| `javax.net.ssl.*` imports; unrelated local lookalike alongside the JDK FQN | Yes | Yes | `StarImport`, `Lookalike` |
| Labeled `return@verify true`; trailing parameter comma | Yes | Yes | `GoMisses` |
| `= (true)`, `return (true)`, backticked `verify` | No | Yes | `GoMisses` |
| Explicit SAM constructor; parenthesized lambda value; labeled return; lambda in parentheses | Yes | Yes | `Lambda` |
| Implicit SAM conversion: JDK static/instance setters, OkHttp builder, named lambda argument | No | Yes | `SamConversion` |
| Logging or local declarations before an all-literal-true result | Yes for plain class members; No for enum, entry, superclass-constructor or secondary-constructor owners | Yes | `Negative`, `LintBodies` |
| Earlier false/nonliteral return followed by literal true | Yes | No; Go-only Lint parity | `Negative`, `LintBodies` |
| Throw, `Nothing`-returning call, or Kotlin precondition before literal true | Yes for plain class members; No for enum, entry, superclass-constructor or secondary-constructor owners | No | `Negative`, `StrictObjects`, `RejectingPaths`, `LintBodies` |
| Multiple literal-true returns; inline nonlocal true return | Yes | Yes | `LintBodies` |
| Direct literal-true override with helper calls, initializer/property calls, another base or secondary constructor | Yes | Yes | `LintBodies` |
| Wrong overload preceding the real literal-true override | Yes, at the actual override | Yes, at the actual override | `Divergence` |

Main's false positives from header mentions, wrong-type/generic overloads, local non-members and
comparisons against true are not part of the preserved true-positive contract. The current Go
class-wide helper-return quirks remain pinned in the base golden (`Factory`), `Negative`
(`AnonymousHelper`, `ExtensionHelper`) and `Divergence` (`CompanionHelper`); FIR excludes them.
They deliberately retain main's class-wide visitor compatibility rather than changing its return order.
FIR-only resolution for SAM aliases and a positional SAM beside a same-named factory is pinned in
`TypeAlias` and `SamLookalike`: Go's file-wide shadow guard intentionally rejects both factory and SAM
calls rather than guessing overload resolution. `Shadow` checks
that an unrelated parameter leaves a real verifier reportable. Imported cross-file lookalikes
remain silent in the compiler test.

Both rules use Android Lint 32.4.1's diagnostic messages, confirmed with `javap` against
`lint-checks-32.4.1.jar`. Lint checks the Apache constructor and a field argument to `setHostnameVerifier`
or `setDefaultHostnameVerifier`. Krit covers Apache field references and constructor references
(`::AllowAllHostnameVerifier`), not just setter arguments. The Go source visitor still misses the
Kotlin imported property reference `::ALLOW_ALL_HOSTNAME_VERIFIER` and Java field method references
(`ALLOW_ALL_HOSTNAME_VERIFIER::verify` and
`SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER::verify`); the Kotlin FIR/Go divergence is pinned in
`AllowAllHostnameVerifier.kt`. These source-only Go gaps remain unchanged here. Krit does not report
imports or custom always-true implementations under this issue ID.

Go requires an explicit import (including aliases and Java static imports) or a fully qualified name
for Apache references; JDK verifier types also resolve `javax.net.ssl.*` with same-file shadow checks.
Shadowed and unresolved lookalikes are silent. FIR checks the resolved constructor or field symbol.
Apache wildcard imports require compiler resolution; Go's Apache shadow exclusions apply file-wide.
Remaining Apache FIR/Go coverage differences are recorded in
`tools/krit-fir/compiler-tests/src/test/data/diagnostic/security/`
goldens with diagnostic markers, explanatory comments and machine-checked `// go-lines:` headers.
`Super` pins secondary `super()` constructor calls and `InheritedField` pins inherited Java statics:
FIR reports them, while Go intentionally remains silent. `EnumShadow`, `CatchShadow` and
`NamedArgument` pin declaration/label exclusions without suppressing real imported field values.

Former custom-verifier FIR golden cases now run as `BadHostnameVerifier` compiler tests and Go tests.
Existing baselines may need regeneration only
for old `AllowAllHostnameVerifier` custom-verifier findings: they become `BadHostnameVerifier`, or
collapse into an existing finding from that rule. In the playground baseline, the sole change is
the finding at `NetworkHelper.kt:35` moving from `AllowAllHostnameVerifier` to `BadHostnameVerifier`.
