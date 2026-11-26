package fixtures.compose

import android.content.Intent
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.material.Button
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect

@Composable
fun PickImage(intent: Intent) {
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.StartActivityForResult()) {}
    SideEffect { launcher.launch(intent, null) }
    Button(onClick = { launcher.launch(intent, null) }) { Text("Pick") }
}

class Launcher {
    fun launch(intent: Intent, options: Any?) = Unit
}

@Composable
fun NotActivityResult(launcher: Launcher, intent: Intent) {
    launcher.launch(intent, null)
}
