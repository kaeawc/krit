# Hostname verifier rules

Both rules are enabled by default, but cover different hazards:

| Rule | Example |
| --- | --- |
| `BadHostnameVerifier` (`android-lint`) | A `javax.net.ssl.HostnameVerifier` with `override fun verify(host: String, session: SSLSession) = true`, or `HostnameVerifier { _, _ -> true }` |
| `AllowAllHostnameVerifier` (`security`) | `org.apache.http.conn.ssl.AllowAllHostnameVerifier()` or `org.apache.http.conn.ssl.SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER` |

Custom implementations no longer receive both findings. `BadHostnameVerifier` runs only in Go,
including when FIR is enabled. Its original class-wide Android Lint visitor, signature matching,
same-file inheritance resolution and `verify` name anchor are retained. The visitor's last return
decides whether the class is complex: logging, declarations, throws and even an earlier `return false`
can be overridden by a final `return true`. Its class-wide overload quirks are retained too; this is
compatibility with the existing rule, not a proof that every execution returns true.

Additional source-proven literal-true methods on direct JDK verifiers and explicit
`HostnameVerifier { _, _ -> true }` SAM constructors retain safe cases previously reported only by
the old `AllowAllHostnameVerifier`. They emit at most one finding per class or SAM constructor.
The additional method check stops at nested owners, functions (including anonymous functions),
initializers, constructors and accessors. Unresolved SAM lookalikes are silent. Implicit SAM
conversions are not inferred from setter names alone.

`AllowAllHostnameVerifier` uses Android Lint 32.4.1's diagnostic messages, confirmed with `javap` against
`lint-checks-32.4.1.jar`. Lint checks the Apache constructor and a field argument to `setHostnameVerifier`
or `setDefaultHostnameVerifier`. Krit deliberately covers all references to the Apache field and constructor
references (`::AllowAllHostnameVerifier`), not just setter arguments. It does not report imports or custom
always-true implementations under this issue ID.

Go requires an explicit import (including aliases and Java static imports) or a fully qualified name
for Apache references; JDK verifier types also resolve `javax.net.ssl.*` with same-file shadow checks.
Shadowed and unresolved lookalikes are silent. FIR checks the resolved constructor or field symbol.
Apache wildcard imports require compiler resolution; Go's Apache shadow exclusions apply file-wide.
Remaining Apache FIR/Go coverage differences are recorded in
`tools/krit-fir/compiler-tests/src/test/data/diagnostic/security/`
goldens with diagnostic markers, explanatory comments and machine-checked `// go-lines:` headers.

Former `BadHostnameVerifier` FIR golden cases now run as Go tests. No FIR checker is registered for
that rule, so FIR cannot suppress its Go findings. Existing baselines may need regeneration only
for old `AllowAllHostnameVerifier` custom-verifier findings: they become `BadHostnameVerifier`, or
collapse into an existing finding from that rule. In the playground baseline, the sole change is
the finding at `NetworkHelper.kt:35` moving from `AllowAllHostnameVerifier` to `BadHostnameVerifier`.
