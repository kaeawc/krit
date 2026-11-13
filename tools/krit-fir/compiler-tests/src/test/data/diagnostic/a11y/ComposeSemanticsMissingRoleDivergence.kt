// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 49
// Where the checker reads resolution and Go reads names and the words of the chain.
package test

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.foundation.clickable as tappable

// An import alias hides the name from Go; the checker resolves the call to the foundation modifier.
@Composable
fun Aliased(onClick: () -> Unit) {
    Row(Modifier.<!ComposeSemanticsMissingRole!>tappable<!> { onClick() }) {}
}

fun roleChanged() {}

// Go searches the chain's text for "semantics" and "role", and finds the words in an unrelated call and in the lambda.
@Composable
fun WordsInChain(onClick: () -> Unit) {
    Row(
        Modifier
            .semantics { contentDescription = "Open" }
            .<!ComposeSemanticsMissingRole!>clickable<!> { roleChanged(); onClick() },
    ) {}
}

// Go exempts internal helpers only previews call; other files can call them too, so the checker reports.
@Composable
internal fun InternalPreviewHelper(onClick: () -> Unit) {
    Row(Modifier.<!ComposeSemanticsMissingRole!>clickable<!> { onClick() }) {}
}

@androidx.compose.ui.tooling.preview.Preview
@Composable
fun InternalHelperPreview() {
    InternalPreviewHelper {}
}

// A project modifier named like the foundation one is not the foundation modifier.
fun Modifier.clickable(label: String, onTap: () -> Unit): Modifier = this

@Composable
fun LocalLookalike(onTap: () -> Unit) {
    Row(Modifier.clickable("Open") { onTap() }) {}
}
