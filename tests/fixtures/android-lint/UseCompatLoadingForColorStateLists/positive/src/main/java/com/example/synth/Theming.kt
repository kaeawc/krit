package com.example.synth

import android.content.Context
import android.content.res.ColorStateList
import android.content.res.Resources
import android.view.View

class Theming(private val context: Context) {
    fun tint(): ColorStateList = context.resources.getColorStateList(android.R.color.primary_text_dark)

    fun tintOf(res: Resources): ColorStateList = res.getColorStateList(android.R.color.primary_text_dark)

    fun tintFor(view: View): ColorStateList = view.resources.getColorStateList(android.R.color.white)
}
