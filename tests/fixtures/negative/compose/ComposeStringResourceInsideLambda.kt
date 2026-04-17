package test
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource

@Composable
fun Example() {
    val label = stringResource(R.string.click_label)
    Button(onClick = { Log.d("TAG", label) }) { Text("Click") }
}
