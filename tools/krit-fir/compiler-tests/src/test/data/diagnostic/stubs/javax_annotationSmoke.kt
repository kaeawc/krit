// Smoke: javax.annotation.CheckReturnValue annotates a Kotlin function.
package stubs

import javax.annotation.CheckReturnValue

@CheckReturnValue
fun checked(): Int = 1

fun useChecked(): Int = checked()
