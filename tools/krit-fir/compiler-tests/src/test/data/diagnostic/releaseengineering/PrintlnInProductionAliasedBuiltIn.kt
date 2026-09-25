// RENDER_DIAGNOSTICS_FULL_TEXT
// Go misses this: it matches the call by the name println / print, and here
// kotlin.io.println is imported under another name. FIR reports it.
package test

import kotlin.io.println as log

fun aliasedBuiltIn() {
    <!PrintlnInProduction!>log("aliased built-in")<!>
}
