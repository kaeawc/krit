package com.example

import android.content.Intent
import androidx.activity.compose.PredictiveBackHandler as Back
import androidx.activity.compose.rememberLauncherForActivityResult as rl
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.Composable

@Composable
fun Aliased(enabled: Boolean, intent: Intent) {
    Back { progress -> }
    androidx.activity.compose.PredictiveBackHandler(enabled, { progress -> })
    val a = rl(ActivityResultContracts.StartActivityForResult()) {}
    a.launch(intent, null)
}
