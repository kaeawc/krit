// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 17, 22, 28, 34, 40
// A Compose Text with a string literal that has letters cannot be translated.
package test

import androidx.compose.foundation.layout.Column
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Header() {
    <!ComposeRawTextLiteral!>Text<!>("Welcome")
}

@Composable
fun NamedText() {
    <!ComposeRawTextLiteral!>Text<!>(text = "Welcome")
}

@Composable
fun Template(name: String) {
    <!ComposeRawTextLiteral!>Text<!>("Hello $name")
}

@Composable
fun NestedInLayout() {
    Column {
        <!ComposeRawTextLiteral!>Text<!>("Settings")
    }
}

@Composable
fun MaterialTwo() {
    androidx.compose.material.<!ComposeRawTextLiteral!>Text<!>("Legacy")
}

class Screen {
    @Composable
    fun Content() {
        <!ComposeRawTextLiteral!>Text<!>("Member")
    }
}
