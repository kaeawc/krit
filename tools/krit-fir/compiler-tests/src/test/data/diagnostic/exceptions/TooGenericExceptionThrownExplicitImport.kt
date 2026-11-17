// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 13, 14, 15
// An explicit import of a generic exception class: each throw constructs the
// generic class, and both report it.
package test

import java.lang.Error
import java.lang.Exception
import java.lang.RuntimeException
import kotlin.Throwable

fun importedException(): Nothing = <!TooGenericExceptionThrown!>throw Exception("explicit import")<!>
fun importedRuntimeException(): Nothing = <!TooGenericExceptionThrown!>throw RuntimeException("explicit import")<!>
fun importedError(): Nothing = <!TooGenericExceptionThrown!>throw Error("explicit import")<!>
fun importedThrowable(): Nothing = <!TooGenericExceptionThrown!>throw Throwable("explicit import")<!>
