package test
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource

@Composable
fun Example() {
    Button(onClick = {
        Log.d("TAG", stringResource(R.string.click_label))
    }) { Text("Click") }
}
