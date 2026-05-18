package com.example

import android.content.Context
import android.util.AttributeSet
import android.widget.Button
import android.widget.TextView

class HardcodedTextActivity {
    private val textView: TextView = TODO()
    private val button: Button = TODO()

    fun setupViews() {
        // Hardcoded literal on a resolved TextView field.
        textView.setText("Hello, world")
        // Button is a TextView subtype — also flagged.
        button.setText("Click me")
    }
}

class CustomLabel(context: Context, attrs: AttributeSet?) : TextView(context, attrs) {
    fun reset() {
        // Bare setText inside a TextView subclass — receiver is `this`.
        setText("Default label")
    }
}
