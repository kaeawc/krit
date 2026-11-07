// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 15, 16, 17
// Divergence (recall): Go resolves an explicit import of a generic exception
// class to a name outside its table (it maps java.lang.Exception to
// kotlin.Exception, and kotlin.Throwable is not listed), so it reports none of
// these throws. Each constructs the generic class.
package test

import java.lang.Error
import java.lang.Exception
import java.lang.RuntimeException
import kotlin.Throwable

fun importedException(): Nothing = throw <!TooGenericExceptionThrown!>Exception("explicit import")<!>
fun importedRuntimeException(): Nothing = throw <!TooGenericExceptionThrown!>RuntimeException("explicit import")<!>
fun importedError(): Nothing = throw <!TooGenericExceptionThrown!>Error("explicit import")<!>
fun importedThrowable(): Nothing = throw <!TooGenericExceptionThrown!>Throwable("explicit import")<!>
