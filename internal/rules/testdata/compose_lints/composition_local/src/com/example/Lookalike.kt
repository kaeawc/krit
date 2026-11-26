package com.example.lookalike

fun <T> compositionLocalOf(factory: () -> T): T = factory()

val LocalLookalike = compositionLocalOf { "Lookalike" }

val LocalLookalikeGetter get() = compositionLocalOf { "Lookalike" }
