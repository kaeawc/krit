// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 17
// Negative: a local function named MutableSharedFlow is not the kotlinx factory.
package test.lookalike

class Bus<T>

@Suppress("FunctionName")
fun <T> MutableSharedFlow(): Bus<T> = Bus()

class EventBus {
    // Go reports these because the text contains "MutableSharedFlow<" and
    // ">()" (or "MutableSharedFlow()"); FIR is correct because the call
    // creates a Bus, not a kotlinx MutableSharedFlow.
    val events = MutableSharedFlow<String>()

    val typed: Bus<Int> = MutableSharedFlow()
}
