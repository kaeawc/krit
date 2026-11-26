package fixtures.compose

import androidx.compose.material.ButtonDefaults
import androidx.compose.material.MaterialTheme
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun M2Screen() {
    Text("hello")
    val colors = ButtonDefaults.buttonColors()
    val primary = MaterialTheme.colors.primary
    println(colors)
    println(primary)
}
