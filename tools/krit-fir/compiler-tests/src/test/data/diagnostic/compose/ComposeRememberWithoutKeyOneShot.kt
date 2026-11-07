// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 31, 32, 33
// A function-typed parameter that the calculation only invokes builds a one-shot holder, so it is not a
// captured value: callers pass a fresh lambda on every recomposition and a key would rebuild the object.
// Data parameters, lambdas that escape, and deferred reads of a data parameter are still reported.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

class Holder(val value: Any?)

class Nav {
    val toolbar: Int = 0
}

fun register(callback: () -> Int): Int = callback()

@Composable
fun InvokeOnly(create: () -> Holder, build: (Int) -> Holder) {
    val a = remember { Holder(create()) }
    val b = remember { build(1) }
    val c = remember { create.invoke() }
    println(a)
    println(b)
    println(c)
}

@Composable
fun StillReported(create: () -> Holder, items: List<Int>, other: () -> Int, nav: Nav) {
    val a = <!ComposeRememberWithoutKey!>remember<!> { Holder(create().value to items) }
    val b = <!ComposeRememberWithoutKey!>remember<!> { register(other) }
    val c = <!ComposeRememberWithoutKey!>remember<!> { Holder({ nav.toolbar }) }
    println(a)
    println(b)
    println(c)
}
