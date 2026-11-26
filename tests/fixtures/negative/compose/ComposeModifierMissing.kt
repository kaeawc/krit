package fixtures.compose

import androidx.compose.foundation.layout.Column
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.glance.GlanceModifier

@Composable
fun Header(title: String, modifier: Modifier = Modifier) {
    Column(modifier) { Text(title) }
}

@Composable
private fun PrivateHeader(title: String) {
    Text(title)
}

@Preview
@Composable
fun HeaderPreview() {
    Header("preview")
}

@Composable
fun Dialog(onDismiss: () -> Unit) {
    AlertDialog(onDismissRequest = onDismiss, confirmButton = { Text("ok") })
}

@Composable
fun Value(): Int {
    Text("x")
    return 1
}

// A GlanceModifier extension receiver is the modifier parameter.
@Composable
fun GlanceModifier.GlanceLabel(title: String) {
    Column { Text(title) }
}
