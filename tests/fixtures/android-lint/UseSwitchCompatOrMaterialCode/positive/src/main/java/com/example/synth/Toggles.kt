package com.example.synth

import android.content.Context
import android.widget.Switch

class Toggles(private val context: Context) {
    private lateinit var wifi: Switch

    fun build(): Switch {
        val bluetooth = Switch(context)
        return bluetooth
    }

    fun bind(airplane: Switch) {
        airplane.isChecked = true
    }
}

class LabeledSwitch(context: Context) : Switch(context)
