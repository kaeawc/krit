// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 26
// Where the checker reads the value of the string and the resolution of the call, and Go reads their text.
package test

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalInspectionMode

// Go reads the escape `\n` as the letter n; the string's value is only a line break.
@Composable
fun LineBreak() {
    Text("\n")
}

// Go leaves a Text with a parenthesized argument alone; the parentheses do not change what is shown.
@Composable
fun Parenthesized() {
    <!ComposeRawTextLiteral!>Text<!>(("Welcome"))
}

// A condition that only contains the inspection read is not an inspection branch.
@Composable
fun CompoundCondition(visible: Boolean) {
    if (LocalInspectionMode.current && visible) {
        <!ComposeRawTextLiteral!>Text<!>("Welcome")
    }
}
