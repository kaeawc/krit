// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// An import alias of kotlin.require.
package test

import kotlin.require as ensure

// Divergence (Go misses): Go matches the written callee name `require`; the
// alias resolves to kotlin.require and the call is require(x != null).
fun aliased(x: Any?) {
    <!UseRequireNotNull!>ensure(x != null)<!>
}
