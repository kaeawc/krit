// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 15, 17
// `import kotlin.emptyArray as arrayOf` renames `emptyArray` to `arrayOf` for
// every unqualified reference in this file. A qualified `kotlin.arrayOf` is
// still the stdlib `arrayOf`: an alias never renames a qualified reference.
package test

import kotlin.emptyArray as arrayOf

annotation class Names4(val values: Array<String> = arrayOf())

// Divergence: the written `arrayOf` is the alias of `emptyArray`, the empty
// counterpart itself, in an annotation argument, an annotation parameter's
// default (line 10) and a plain call. Go reports each by its written name.
@Names4(arrayOf())
fun aliased() {
    val a: Array<String> = arrayOf()
    println(a)
}

// The qualified calls are the stdlib `arrayOf` despite the alias. Go misses
// them: the callee is a navigation expression.
@Names4(kotlin.<!UseEmptyCounterpart!>arrayOf<!>())
fun qualified() {
    val a = kotlin.<!UseEmptyCounterpart!>arrayOf<!><String>()
    println(a)
}

@Names4(values = kotlin.<!UseEmptyCounterpart!>arrayOf<!><String>())
fun qualifiedNamed() {
}
