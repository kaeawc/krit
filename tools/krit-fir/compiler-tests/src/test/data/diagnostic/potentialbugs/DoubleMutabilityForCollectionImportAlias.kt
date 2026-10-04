// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positives Go misses: an import alias of a mutable collection type. Go reads
// the alias name (`ML`, `JavaHashMap`), which is neither a listed type name
// nor a listed factory name.
package test

import kotlin.collections.MutableList as ML
import java.util.HashMap as JavaHashMap

fun <T> buildAliased(): ML<T> = mutableListOf()

class ImportAlias {
    <!DoubleMutabilityForCollection!>var<!> importAliased: ML<String> = buildAliased()

    <!DoubleMutabilityForCollection!>var<!> javaAliased: JavaHashMap<String, Int> = JavaHashMap()

    <!DoubleMutabilityForCollection!>var<!> inferredAliased = JavaHashMap<String, Int>()
}
