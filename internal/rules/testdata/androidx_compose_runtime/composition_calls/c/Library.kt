package c

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReusableContent
import androidx.compose.runtime.key
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

@Composable
fun Material(scope: CoroutineScope, list: List<Int>) {
    Scaffold(topBar = { scope.launch {} }, bottomBar = { scope.launch {} }) { padding -> scope.launch {} }
    Button(onClick = { scope.launch {} }) { scope.launch {} }
    TextButton(onClick = { scope.launch {} }, content = { scope.launch {} })
    IconButton(onClick = { scope.launch {} }) { scope.launch {} }
    Surface { scope.launch {} }
    Card { scope.launch {} }
    MaterialTheme { scope.launch {} }
    Row { scope.launch {} }
    BoxWithConstraints { scope.launch {} }
    key(1) { scope.launch {} }
    ReusableContent(1) { scope.launch {} }
    LazyRow {
        items(list) { scope.launch {} }
        item(key = 1) { scope.launch {} }
    }
    LazyVerticalGrid(columns = GridCells.Fixed(2)) {
        item { scope.launch {} }
    }
    Box(Modifier) { scope.launch {} }
    Text("x", modifier = Modifier)
}

object Modifier : androidx.compose.ui.Modifier by androidx.compose.ui.Modifier
