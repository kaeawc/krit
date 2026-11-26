package com.example

import androidx.compose.runtime.compositionLocalOf

val LocalCommented get() = /* explanation */ compositionLocalOf { 0 }

val LocalAnnotated: Any
    @Suppress("unused") get() = compositionLocalOf { 1 }
