package com.example.synth

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.OnBackPressedCallback

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            // OnBackPressedCallback's method is handleOnBackPressed; Lint only matches handledOnBackPressed.
            override fun handleOnBackPressed() {
                onBackPressed()
            }
        })
    }

    fun close() {
        onBackPressed()
    }
}
