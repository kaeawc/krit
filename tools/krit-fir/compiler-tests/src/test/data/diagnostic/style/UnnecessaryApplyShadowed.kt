// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A user-declared or aliased apply is not kotlin.apply.
package test

import kotlin.apply as ap

class Q { fun foo() {}; var v = 0 }
fun <T> T.apply(block: T.() -> Unit): T = this

fun custom(q: Q) {
    q.apply { foo() } // E01 user apply
    q.ap { foo() } // E02 alias
    q.also { foo2() } // E03 filler
}
fun foo2() {}
