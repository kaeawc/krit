package com.example.synth

import android.content.Context
import android.content.res.ColorStateList
import androidx.appcompat.content.res.AppCompatResources
import androidx.core.content.ContextCompat

class Palette {
    fun getColorStateList(id: Int): ColorStateList? = null
}

class Theming(private val context: Context) {
    private val resources = Palette()

    fun compat(): ColorStateList? = AppCompatResources.getColorStateList(context, android.R.color.white)

    fun core(): ColorStateList? = ContextCompat.getColorStateList(context, android.R.color.white)

    // Context.getColorStateList is not the deprecated Resources method.
    fun fromContext(): ColorStateList = context.getColorStateList(android.R.color.white)

    fun themed(): ColorStateList = context.resources.getColorStateList(android.R.color.white, context.theme)

    fun shadowed(): ColorStateList? = resources.getColorStateList(android.R.color.white)
}
