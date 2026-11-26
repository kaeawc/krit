package fixtures.compose

import androidx.compose.ui.Modifier
import androidx.compose.ui.composed

class Builder {
    fun composed(block: () -> Unit) = block()
}

fun Builder.build() = composed { }

fun lookalike(builder: Builder) = builder.composed { }

// Modifier.composed { } in a comment
val text = "Modifier.composed { }"

fun Modifier.ok(): Modifier = this
