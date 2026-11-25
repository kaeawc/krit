package com.example.synth

import android.content.res.ColorStateList
import android.graphics.PorterDuff
import android.widget.TextView
import androidx.core.widget.TextViewCompat

class Chipset {
    var compoundDrawableTintList: ColorStateList? = null

    fun setCompoundDrawableTintMode(mode: PorterDuff.Mode?) {}
}

class Labels(private val label: TextView, private val chips: Chipset) {
    fun tint(colors: ColorStateList) {
        TextViewCompat.setCompoundDrawableTintList(label, colors)
        TextViewCompat.setCompoundDrawableTintMode(label, PorterDuff.Mode.SRC_IN)
        chips.compoundDrawableTintList = colors
        chips.setCompoundDrawableTintMode(null)
        val current = label.compoundDrawableTintList
    }
}
