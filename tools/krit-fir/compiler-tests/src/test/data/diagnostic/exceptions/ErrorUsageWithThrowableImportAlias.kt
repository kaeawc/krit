// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: a top-level function from another package imported under the
// alias `error` wins over the default import of kotlin.error.
// Go reports this because it matches the bare name `error`; FIR is correct
// because the call resolves to kotlin.io.println.
package test

import kotlin.io.println as error

fun importedFromOtherPackage(e: Exception) {
    error(e)
}
