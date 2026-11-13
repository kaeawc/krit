// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 59, 61, 111
// Constant-keyed effects that are deliberate: callbacks, one-shot bodies, documented one-shots, and
// preview or testing scaffolding.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

fun fetch(value: Any?) {}
fun trackScreenView(value: Any?) {}
fun showToast(value: Any?) {}
fun open(value: Any?) {}
fun scrollTo(value: Any?) {}

class Handler {
    fun run() {}
}

class Host {
    fun showSnackbar(value: Any?) {}
}

class Event

// Callbacks and event sinks are read through the latest value, so a constant key is fine.
@Composable
fun Callbacks(onReady: () -> Unit, eventSink: (Event) -> Unit, handler: Handler, later: () -> Unit) {
    LaunchedEffect(Unit) {
        onReady()
        eventSink(Event())
        handler.run()
        later()
    }
}

@Composable
fun OneShotBodies(userId: String, shouldOpen: Boolean, host: Host, message: String) {
    LaunchedEffect(Unit) {
        trackScreenView(userId)
    }
    LaunchedEffect(Unit) {
        if (shouldOpen) { open(userId) }
    }
    LaunchedEffect(Unit) {
        showToast(userId)
    }
    LaunchedEffect(Unit) {
        host.showSnackbar(message)
    }
}

// A comment directly above the effect that calls it one-shot excuses it; a comment that does not, or a
// blank statement between, does not.
@Composable
fun Documented(name: String, other: String, third: String) {
    // One-shot: records the screen the first time it is shown.
    LaunchedEffect(Unit) { fetch(name) }
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) { fetch(other) }
    // Fetches the data.
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) { fetch(third) }
}

@Composable
fun DocumentedFirst(target: String) {
    // Runs once when the screen opens.
    LaunchedEffect(Unit) {
        scrollTo(target)
    }
}

// Preview and testing scaffolding is exempt; a helper that production code also calls is not. An internal
// helper is reported even if only this file's previews call it (Go exempts it), as other files may call it.
@Composable
private fun PreviewOnlyHelper(dataset: List<Int>) {
    LaunchedEffect(Unit) { fetch(dataset) }
}

@Composable
internal fun InternalPreviewOnlyHelper(dataset: List<Int>) {
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) { fetch(dataset) }
}

@Preview
@Composable
fun ScaffoldPreview() {
    PreviewOnlyHelper(emptyList())
    InternalPreviewOnlyHelper(emptyList())
}

@Composable
fun SamplePanel(dataset: List<Int>) {
    LaunchedEffect(Unit) { fetch(dataset) }
}

@VisibleForTesting
@Composable
fun TestingChart(dataset: List<Int>) {
    LaunchedEffect(Unit) { fetch(dataset) }
}

class FakeScreens {
    @Composable
    fun Screen(dataset: List<Int>) {
        LaunchedEffect(Unit) { fetch(dataset) }
    }
}

@Composable
private fun SharedHelper(dataset: List<Int>) {
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) { fetch(dataset) }
}

@Preview
@Composable
fun SharedPreview() {
    SharedHelper(emptyList())
}

@Composable
fun ProductionScreen() {
    SharedHelper(emptyList())
}

annotation class Preview
annotation class VisibleForTesting
