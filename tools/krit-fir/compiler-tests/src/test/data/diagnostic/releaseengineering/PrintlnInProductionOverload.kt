// RENDER_DIAGNOSTICS_FULL_TEXT
// Go misses the String call: the file declares a function named println, so
// Go skips every bare println in it. That overload takes an Int, so
// println("x") falls through to kotlin.io.println, which FIR reports. The Int
// call resolves to the overload and prints nothing, so neither reports it.
package test

fun println(value: Int): Int = value

fun overloaded() {
    <!PrintlnInProduction!>println("does not fit the Int overload")<!>
    require(println(1) == 1)
}
