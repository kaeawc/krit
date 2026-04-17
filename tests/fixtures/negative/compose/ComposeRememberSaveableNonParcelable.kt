package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.saveable.rememberSaveable

@Composable
fun Example() {
    val count = rememberSaveable { 0 }
}
