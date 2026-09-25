// RENDER_DIAGNOSTICS_FULL_TEXT
// Go misses this: it treats any import aliased as println as a shadow of the
// built-in. The alias here is kotlin.io.print itself, so the call is console
// output and FIR reports it.
package test

import kotlin.io.print as println

fun aliasedPrint() {
    <!PrintlnInProduction!>println("kotlin.io.print under the name println")<!>
}
