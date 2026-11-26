package com.example.wear

import androidx.compose.material.MaterialTheme
import androidx.compose.material3.MaterialTheme as M3Theme
import androidx.compose.material.Colors
import androidx.wear.compose.material3.MaterialTheme as WearTheme

object Themes {
    val names = listOf(MaterialTheme::class, M3Theme::class, Colors::class, WearTheme::class)
}
