package com.example.wear

import androidx.wear.compose.material3.MaterialTheme
import androidx.wear.compose.material.MaterialTheme as WearMaterial2Theme
import com.example.material.MaterialTheme as LocalTheme

// import androidx.compose.material.MaterialTheme

object Themes {
    val names = listOf(MaterialTheme::class, WearMaterial2Theme::class, LocalTheme::class)
}
