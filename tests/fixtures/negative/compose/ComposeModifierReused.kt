package fixtures.compose

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Labels(modifier: Modifier = Modifier) {
    Column(modifier) {
        Text("a", Modifier.fillMaxWidth())
    }
}

@Composable
fun Either(flag: Boolean, modifier: Modifier = Modifier) {
    if (flag) {
        Text("a", modifier = modifier)
    } else {
        Text("b", modifier = modifier)
    }
}
