package com.example.synth

import androidx.activity.ComponentActivity

class MainActivity : ComponentActivity() {
    // Lint's OnBackPressedDetector matches a method with exactly this name.
    fun handledOnBackPressed() {
        onBackPressed()
    }
}
