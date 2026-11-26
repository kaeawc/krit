package fixtures.compose

import androidx.compose.material.*
import androidx.compose.material.icons.Icons
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun M3Screen() {
    Text("m3")
    val icon = Icons.Default
    val alpha = MaterialTheme.colors.primary.copy(alpha = 0.5f)
    val color = LocalContentColor.current
    println(icon)
    println(alpha)
    println(color)
    Divider()
    val text = "androidx.compose.material.Text"
    println(text)
}

@Composable
fun Divider() {}
