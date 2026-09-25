// Smoke: read the Android Context from composition.
package stubs

import android.content.Context
import android.widget.Toast
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext

@Composable
fun ToastButton() {
    val context: Context = LocalContext.current
    Button(onClick = { Toast.makeText(context, "hi", Toast.LENGTH_SHORT).show() }) {
        Text("Toast")
    }
}
