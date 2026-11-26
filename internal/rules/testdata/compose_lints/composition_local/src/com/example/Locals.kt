package com.example

import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.staticCompositionLocalOf

private val LocalApple = staticCompositionLocalOf<String> { "Apple" }
internal val LocalPlum: Any = staticCompositionLocalOf { "Plum" }
val LocalPrune = compositionLocalOf { "Prune" }

/** Documented local. */
@Suppress("Unused")
val LocalDocumented = (compositionLocalOf { "Kiwi" })

// The allowlist in lint.xml names these two.
val LocalBanana = staticCompositionLocalOf<String> { "Banana" }
val LocalPotato = compositionLocalOf { "Potato" }

var LocalMutable = compositionLocalOf { "Mutable" }

val LocalQualified = androidx.compose.runtime.compositionLocalOf { "Qualified" }

object Holder {
    val LocalInObject = compositionLocalOf { 1 }

    val LocalGetterInObject
        get() = compositionLocalOf { 2 }
}

class Owner {
    val LocalMember = staticCompositionLocalOf { 3 }

    companion object {
        val LocalCompanion = staticCompositionLocalOf { 4 }
    }
}

fun build() {
    val LocalNested = compositionLocalOf { 5 }
    println(LocalNested)
}

val LocalGetter get() = compositionLocalOf { "Getter" }

val LocalBlockGetter: Any
    get() {
        return compositionLocalOf { "Block" }
    }

val LocalParenGetter: Any
    get() {
        return (staticCompositionLocalOf { "Paren" })
    }

val LocalTwoStatements: Any
    get() {
        println("first")
        return compositionLocalOf { "Second" }
    }

val NotALocal = listOf(1)

val NotALocalGetter get() = listOf(2)
