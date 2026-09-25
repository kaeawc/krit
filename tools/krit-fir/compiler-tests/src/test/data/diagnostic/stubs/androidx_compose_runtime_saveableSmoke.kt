// Smoke: rememberSaveable with delegated state, keys, and a custom Saver.
package stubs

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.Saver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue

data class SavedPoint(val x: Int, val y: Int)

val SavedPointSaver: Saver<SavedPoint, Any> = Saver(
    save = { listOf(it.x, it.y) },
    restore = { value ->
        val parts = value as List<*>
        SavedPoint(parts[0] as Int, parts[1] as Int)
    },
)

@Composable
fun SaveableSmoke() {
    var text by rememberSaveable { mutableStateOf("") }
    val length = rememberSaveable(text) { text.length }
    val point = rememberSaveable(saver = SavedPointSaver) { SavedPoint(1, 2) }
    text = "x$length$point"
}
