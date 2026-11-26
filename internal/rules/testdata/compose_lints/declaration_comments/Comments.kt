package com.example

import androidx.compose.runtime.compositionLocalOf
/** KDoc right after imports. */
val LocalA = compositionLocalOf { 1 }

// line comment, no blank
val LocalB = compositionLocalOf { 1 }

// line comment then blank

val LocalC = compositionLocalOf { 1 }

/** KDoc then blank */

val LocalD = compositionLocalOf { 1 }

// first
// second
val LocalE = compositionLocalOf { 1 }

/* block */
val LocalF = compositionLocalOf { 1 }

/** kdoc */
// then line
val LocalG = compositionLocalOf { 1 }

val x = 1 // trailing
val LocalH = compositionLocalOf { 1 }

object O {
    // member comment
    val LocalJ = compositionLocalOf { 1 }

    /** member kdoc */
    @Suppress("X")
    val LocalK = compositionLocalOf { 1 }
}
