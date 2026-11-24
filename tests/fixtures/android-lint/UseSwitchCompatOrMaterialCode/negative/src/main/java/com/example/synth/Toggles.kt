package com.example.synth

import android.content.Context
import android.widget.CompoundButton
import androidx.appcompat.widget.SwitchCompat
import com.google.android.material.materialswitch.MaterialSwitch

class Toggles(private val context: Context) {
    private lateinit var wifi: SwitchCompat
    private var cellular: MaterialSwitch? = null

    fun bind(button: CompoundButton) {
        val compat = SwitchCompat(context)
        // val legacy: android.widget.Switch
    }
}
