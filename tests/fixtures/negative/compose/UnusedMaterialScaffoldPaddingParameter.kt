package fixtures.compose

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Scaffold
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Screen(names: List<String>) {
    Scaffold { Column(Modifier.padding(it)) { Text("Hello") } }
    Scaffold { innerPadding -> Column(Modifier.padding(innerPadding)) {} }
    Scaffold { innerPadding -> names.forEach { Text(it + innerPadding) } }
    Scaffold(topBar = { Text("Title") }) { padding -> Text("$padding") }
}

@Composable
fun Other() {
    androidx.compose.material3.Scaffold { Text("material3 has its own check") }
}
