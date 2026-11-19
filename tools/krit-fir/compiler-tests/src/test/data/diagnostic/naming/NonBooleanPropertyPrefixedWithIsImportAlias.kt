// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: import aliases of kotlin.Boolean. Kept apart from
// NonBooleanPropertyPrefixedWithIsBooleanForms.kt because aliasing the import
// hides the simple name `Boolean` from kotlin.Boolean in this file. Go reads
// the file's import aliases, so it does not report these either.
package test

import kotlin.Boolean as Flag2

class ImportAlias {
    val isImportAliased: Flag2 = true

    val isImportAliasedNullable: Flag2? = null
}
