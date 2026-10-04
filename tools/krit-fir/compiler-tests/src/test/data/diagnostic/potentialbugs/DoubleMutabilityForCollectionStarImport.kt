// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18
// Positives Go misses: any star import other than kotlin.collections.* or
// java.util.* makes Go treat every built-in collection name as possibly
// shadowed, so it ignores the declared types below. Nothing in
// java.util.concurrent is named MutableList, MutableMap, or ArrayList: they
// are still the kotlin.collections / java.util types. Go still reports the
// ArrayList line through its factory-name fallback.
package test

import java.util.concurrent.*

fun <T> build(): MutableList<T> = mutableListOf()

class StarImport {
    <!DoubleMutabilityForCollection!>var<!> list: MutableList<String> = build()

    <!DoubleMutabilityForCollection!>var<!> arrayList: ArrayList<String> = ArrayList()

    val executor: Executor? = null
}
