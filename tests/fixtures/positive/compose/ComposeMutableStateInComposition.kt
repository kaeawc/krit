package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.mutableStateOf

@Composable
fun Counter() {
    val count = mutableStateOf(0)
    Text(count.value.toString())
}
