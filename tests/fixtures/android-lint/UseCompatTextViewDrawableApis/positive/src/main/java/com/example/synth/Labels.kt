package com.example.synth

import android.content.Context
import android.content.res.ColorStateList
import android.graphics.PorterDuff
import android.widget.Button
import android.widget.TextView

class Labels(private val label: TextView, private val action: Button) {
    fun tint(colors: ColorStateList) {
        label.setCompoundDrawableTintList(colors)
        action.compoundDrawableTintMode = PorterDuff.Mode.SRC_IN
        label.apply { setCompoundDrawableTintMode(PorterDuff.Mode.MULTIPLY) }
    }
}

class BadgeView(context: Context) : TextView(context) {
    fun highlight(colors: ColorStateList) {
        compoundDrawableTintList = colors
    }
}
