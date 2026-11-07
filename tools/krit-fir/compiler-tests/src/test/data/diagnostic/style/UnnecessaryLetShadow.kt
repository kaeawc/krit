// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A member extension named let is not kotlin.let, so detekt (which resolves the call) leaves it alone.
package test

class A {
    fun foo(): Int = 1
}

class Shadow {
    fun <T, R> T.let(block: (T) -> R): R = block(this)

    fun member(a: A): Int = a.let { it.foo() }
}

fun <T, R> T.myLet(block: (T) -> R): R = block(this)

fun userExtension(a: A): Int = a.myLet { it.foo() }
