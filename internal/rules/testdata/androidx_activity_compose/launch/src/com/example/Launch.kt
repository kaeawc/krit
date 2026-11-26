package com.example

import androidx.activity.compose.ManagedActivityResultLauncher
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.ActivityResult
import androidx.activity.result.ActivityResultLauncher
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.material.Button
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.SideEffect
import android.content.Intent

@Composable
fun Screen(intent: Intent) {
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.StartActivityForResult()) {}
    launcher.launch(intent)
    SideEffect { launcher.launch(intent) }
    LaunchedEffect(Unit) { launcher.launch(intent) }
    Button(onClick = { launcher.launch(intent) }) { Text("x") }
    if (true) {
        launcher.launch(intent)
    }
    Text("a").also { launcher.launch(intent) }
}

@Composable
fun Typed(launcher: ManagedActivityResultLauncher<Intent, ActivityResult>, intent: Intent) {
    launcher.launch(intent)
    launcher.launch(intent, null)
}

@Composable
fun Plain(launcher: ActivityResultLauncher<Intent>, intent: Intent) {
    launcher.launch(intent)
}

fun notComposable(launcher: ManagedActivityResultLauncher<Intent, ActivityResult>, intent: Intent) {
    launcher.launch(intent)
}

@Composable
fun Lambdas(intent: Intent) {
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.StartActivityForResult()) {}
    val content: @Composable () -> Unit = { launcher.launch(intent) }
    val callback: () -> Unit = { launcher.launch(intent) }
}

@Composable
fun TwoArg(intent: Intent) {
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.StartActivityForResult()) {}
    launcher.launch(intent, null)
    launcher.launch(intent, options = null)
    SideEffect { launcher.launch(intent, null) }
    LaunchedEffect(Unit) { launcher.launch(intent, null) }
    Button(onClick = { launcher.launch(intent, null) }) { Text("x") }
    if (true) {
        launcher.launch(intent, null)
    }
    Text("a").also { launcher.launch(intent, null) }
    val content: @Composable () -> Unit = { launcher.launch(intent, null) }
    val callback: () -> Unit = { launcher.launch(intent, null) }
}

fun notComposableTwoArg(launcher: ManagedActivityResultLauncher<Intent, ActivityResult>, intent: Intent) {
    launcher.launch(intent, null)
}

@Composable
fun PlainTwoArg(launcher: ActivityResultLauncher<Intent>, intent: Intent) {
    launcher.launch(intent, null)
}
