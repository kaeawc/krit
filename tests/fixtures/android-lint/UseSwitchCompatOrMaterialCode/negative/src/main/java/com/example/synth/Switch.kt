package com.example.synth

// A local class named Switch is not android.widget.Switch.
class Switch(val on: Boolean)

class Panel {
    private val main: Switch = Switch(true)
}
