// Smoke: the RxJava 3 CheckReturnValue annotation marks a Kotlin function.
package stubs

import io.reactivex.rxjava3.annotations.CheckReturnValue

@CheckReturnValue
fun rxChecked(): Int = 1

fun useRxChecked(): Int = rxChecked()
