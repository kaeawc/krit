// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 16
// Go finding FIR drops: the property IS a Boolean, so Go's message
// ("Non-Boolean property ...") is false of the code. Kept apart from
// NonBooleanPropertyPrefixedWithIsDivergence.kt because aliasing the import
// hides the simple name `Boolean` from kotlin.Boolean in this file.
package test

import kotlin.Boolean as Flag2

class ImportAlias {
    // Go reports this: the declared type text is `Flag2`, not `Boolean`, but it
    // is an import alias of kotlin.Boolean.
    val isImportAliased: Flag2 = true

    val isImportAliasedNullable: Flag2? = null
}
