// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergences from Go, which keeps a function whenever its name shows up elsewhere in the file.
package test.unusedprivatefunctiondivergence

import kotlin.reflect.KProperty

// Go keeps this: the name appears in another class's method call, but that call resolves elsewhere.
class Other {
    fun lookalike() = 1
}

private fun <!UnusedPrivateFunction!>lookalike<!>() = 2

fun callOther(other: Other) = other.lookalike()

// Go keeps the overload that nothing calls, because the name is called with other arguments.
private fun <!UnusedPrivateFunction!>over<!>(a: Int) = a

private fun over(a: String) = a.length

fun callOver() = over("x")

// Go keeps this: a local variable has the same name.
private fun <!UnusedPrivateFunction!>shadowed<!>() = 1

fun useShadow(): Int {
    val shadowed = 3
    return shadowed
}

// Go keeps a private operator function whenever its symbol shows up anywhere in the file.
class Operators {
    private operator fun <!UnusedPrivateFunction!>plus<!>(other: Operators) = this

    fun go() = 1 + 2
}
