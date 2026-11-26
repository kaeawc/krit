package fixtures.compose

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Screen() {
    Scaffold { Column(Modifier.padding(it)) { Text("Hello") } }
    Scaffold { innerPadding -> Column(Modifier.padding(innerPadding)) {} }
    androidx.compose.material.Scaffold { Text("material has its own check") }
}

class Local {
    @Composable
    fun Scaffold(content: @Composable (Int) -> Unit) = content(0)

    @Composable
    fun Use() {
        Scaffold { Text("member Scaffold wins over the import") }
    }
}
