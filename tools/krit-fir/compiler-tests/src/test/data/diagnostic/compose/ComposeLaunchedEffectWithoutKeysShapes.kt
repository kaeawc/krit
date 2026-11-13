// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 84, 90, 106
// Read shapes and comment placements: which reads count as data, and which comments excuse the effect.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

fun fetch(value: Any?) {}

class Callbacks(val refresh: () -> Unit)

class Controller {
    fun start() {}
}

// An operator read is a data read, not a method receiver.
@Composable
fun IndexRead(items: List<Int>) {
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) {
        fetch(items[0])
    }
}

// A string template and a bound callable reference read the parameter. Go reports neither, so these are
// FIR-only true positives.
@Composable
fun TemplateRead(name: String, controller: Controller) {
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) {
        fetch("hello $name")
    }
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) {
        fetch(controller::start)
    }
}

// A property of function type is invoked through the parameter, which is only a receiver.
@Composable
fun InvokedProperty(callbacks: Callbacks) {
    LaunchedEffect(Unit) {
        callbacks.refresh()
    }
}

// A safe call and an extension call keep the parameter a receiver.
@Composable
fun SafeReceiver(controller: Controller?) {
    LaunchedEffect(Unit) {
        controller?.start()
    }
}

// A block comment and a KDoc above the effect excuse it too.
@Composable
fun BlockComments(first: String, second: String) {
    /* Runs once when shown. */
    LaunchedEffect(Unit) { fetch(first) }
    /** Fires once at startup. */
    LaunchedEffect(Unit) { fetch(second) }
}

// A blank line between the comment and the effect does not break the link.
@Composable
fun BlankLineBetween(value: String) {
    // One-shot request.

    LaunchedEffect(Unit) { fetch(value) }
}

// A comment on the first line of a lambda body counts as above its first statement.
@Composable
fun FirstInLambda(value: String, show: Boolean) {
    if (show) {
        // Only once per screen.
        LaunchedEffect(Unit) { fetch(value) }
    }
}

// A one-shot comment above an earlier statement does not carry over.
@Composable
fun EarlierComment(value: String) {
    // Runs once.
    fetch(0)
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) { fetch(value) }
}

// Only a top-level `if` makes the body a guarded one-shot; a nested one does not.
@Composable
fun NestedIf(value: String, flag: Boolean) {
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) {
        fetch(1)
        fetch(listOf(value).map { if (flag) it else "" })
    }
}

@Composable
fun GuardedLast(value: String, flag: Boolean) {
    LaunchedEffect(Unit) {
        if (flag) fetch(value)
    }
}

// A when is not an if.
@Composable
fun WhenBody(value: String) {
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) {
        when (value) {
            "a" -> fetch(1)
            else -> fetch(2)
        }
    }
}
