package com.example.synth

import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts

class PickerActivity : ComponentActivity() {
    private val pick = registerForActivityResult(ActivityResultContracts.GetContent()) { }
}
