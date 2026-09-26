// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positives Go misses: each `var` below has a mutable collection type that
// the declaration's text does not spell as a listed type name or a listed
// factory call, so Go reports none of them.
package test

import kotlin.properties.Delegates

typealias Names = MutableList<String>

fun names(): Names = mutableListOf()

fun <T> build(): MutableList<T> = mutableListOf()

class Recall {
    // Inferred from a call Go does not list.
    <!DoubleMutabilityForCollection!>var<!> applied = mutableListOf<String>().apply { add("a") }

    <!DoubleMutabilityForCollection!>var<!> copied = listOf("a").toMutableList()

    <!DoubleMutabilityForCollection!>var<!> built = build<String>()

    // A type alias of MutableList.
    <!DoubleMutabilityForCollection!>var<!> aliased: Names = names()

    // A delegated property whose type is inferred from the delegate.
    <!DoubleMutabilityForCollection!>var<!> delegated by Delegates.notNull<MutableList<String>>()
}

// A primary-constructor `var` is a property with a mutable collection type;
// Go visits property declarations only.
class Constructor(<!DoubleMutabilityForCollection!>var<!> items: MutableList<String>, val fixed: MutableList<String>)

data class State(
    <!DoubleMutabilityForCollection!>var<!> tags: HashSet<String>,
)
