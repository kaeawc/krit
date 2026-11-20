# Hostname verifier rules

Both rules are enabled by default, but cover different hazards:

| Rule | Example |
| --- | --- |
| `BadHostnameVerifier` (`android-lint`) | A `javax.net.ssl.HostnameVerifier` with `override fun verify(host: String, session: SSLSession) = true`, or `HostnameVerifier { _, _ -> true }` |
| `AllowAllHostnameVerifier` (`security`) | `org.apache.http.conn.ssl.AllowAllHostnameVerifier()` or `org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER` |

Custom implementations no longer receive both findings. `BadHostnameVerifier` handles named, anonymous,
local, inherited and aliased verifier classes, plus SAM constructors. Go accepts literal-true expressions
and method bodies whose returns are all literal true, including logging, local declarations or throws.
FIR currently requires a single literal-true return; this difference is pinned in the goldens.
Nonliteral returns, non-verifier
owners, wrong parameter types, vararg/generic overloads, extensions and local helper functions are exempt.

`AllowAllHostnameVerifier` uses Android Lint 32.4.1's diagnostic messages, confirmed with `javap` against
`lint-checks-32.4.1.jar`. Lint checks the Apache constructor and a field argument to `setHostnameVerifier`
or `setDefaultHostnameVerifier`. Krit deliberately covers all references to the Apache field and constructor
references (`::AllowAllHostnameVerifier`), not just setter arguments. It does not report imports or custom
always-true implementations under this issue ID.

Go requires an explicit import (including aliases and Java static imports) or a fully qualified name
for Apache references; JDK verifier types also resolve `javax.net.ssl.*` with same-file shadow checks.
Shadowed and unresolved lookalikes are silent. FIR checks the resolved constructor or field symbol.
Apache wildcard imports, aliased parameter/constructor types, qualified or cross-file inheritance, enum-entry
inheritance and implicit SAM conversions need compiler resolution. Go's same-file inheritance search is
bounded and accepts only unambiguous visible simple-name bases; its shadow exclusions apply file-wide.
Remaining FIR/Go coverage differences are recorded in
`tools/krit-fir/compiler-tests/src/test/data/diagnostic/security/`
goldens with diagnostic markers, explanatory comments and machine-checked `// go-lines:` headers.

Existing baselines for custom verifiers may need regeneration: their issue ID is now `BadHostnameVerifier`.
Go retains the `verify` name anchor; FIR anchors at the first annotation or modifier, so annotated
or multiline declarations can have different lines (pinned in `BadHostnameVerifier.kt`).
