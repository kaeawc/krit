// Smoke: errorprone CheckReturnValue and CanIgnoreReturnValue annotate Kotlin functions.
package stubs

import com.google.errorprone.annotations.CanIgnoreReturnValue
import com.google.errorprone.annotations.CheckReturnValue

@CheckReturnValue
fun errorProneChecked(): Int = 1

@CanIgnoreReturnValue
fun errorProneIgnorable(): Int = 1

fun useErrorProne(): Int = errorProneChecked() + errorProneIgnorable()
