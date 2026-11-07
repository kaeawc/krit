// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 52, 74, 100
// Keyless remember calls that are intentional: outside a composable, seeding
// local state from a parameter, parameters named initial* / default*, and state
// the function re-syncs from the parameter elsewhere. A keyless remember that
// reads another parameter is still reported.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember

fun use(x: Any?) {}

// Reported: the memo never updates when `seed` or `second` changes.
@Composable
fun Reported(seed: Int, second: String) {
    val v = <!ComposeRememberWithoutKey!>remember<!> { seed + second.length }
    use(v)
}

// Only composables recompose, so a plain function cannot go stale.
fun NotComposable(seed: Int) {
    val v = remember { seed + 1 }
    use(v)
}

// A state-seeding factory copies the parameter into local state on purpose.
@Composable
fun Seeds(count: Int, items: List<String>) {
    val state = remember { mutableStateOf(count) }
    val intState = remember { mutableIntStateOf(count) }
    val list = remember { mutableStateListOf(items.first()) }
    use(state)
    use(intState)
    use(list)
}

// Parameters named initial* / default* are seeds by convention.
@Composable
fun Seeded(initialCount: Int, defaultLabel: String) {
    val v = remember { initialCount + defaultLabel.length }
    use(v)
}

// The first parameter that is not a seed is the one reported.
@Composable
fun MixedSeed(initialCount: Int, size: Int) {
    val v = <!ComposeRememberWithoutKey!>remember<!> { initialCount + size }
    use(v)
}

// The state is re-synced from the parameter in an effect or an assignment.
@Composable
fun ResyncEffect(value: String) {
    var text = remember { value.trim() }
    LaunchedEffect(value) { text = value.trim() }
    use(text)
}

@Composable
fun ResyncAssignment(value: String) {
    var text = remember { value.trim() }
    if (text != value) text = value.trim()
    use(text)
}

// An effect that never mentions the parameter does not re-sync the state.
@Composable
fun NoResync(value: String) {
    val text = <!ComposeRememberWithoutKey!>remember<!> { value.trim() }
    LaunchedEffect(Unit) { use(text) }
}

// Preview and testing scaffolding is exempt; a private helper that production code also calls is not.
@Composable
private fun PreviewOnlyHelper(dataset: List<Int>) {
    val series = remember { dataset.size }
    use(series)
}

@Preview
@Composable
fun ScaffoldPreview() {
    PreviewOnlyHelper(emptyList())
}

@VisibleForTesting
@Composable
fun TestingChart(dataset: List<Int>) {
    val series = remember { dataset.size }
    use(series)
}

@Composable
private fun SharedHelper(dataset: List<Int>) {
    val series = <!ComposeRememberWithoutKey!>remember<!> { dataset.size }
    use(series)
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
