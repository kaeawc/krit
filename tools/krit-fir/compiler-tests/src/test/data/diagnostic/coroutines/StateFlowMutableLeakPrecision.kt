// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 21, 24, 27, 31, 36, 37, 43, 44, 56, 66, 76
// Deliberate precision differences from the Go rule, which reports any public
// property whose source text contains "MutableStateFlow". None of these
// properties exposes a MutableStateFlow type, so none is reported here.
package test

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

interface Listener {
    fun onEvent()
}

class ViewModel {
    // Declared read-only: the initializer mentions MutableStateFlow, the type does not.
    val readOnly: StateFlow<Int> = MutableStateFlow(0)

    // asStateFlow() returns the read-only view.
    val constant = MutableStateFlow(1).asStateFlow()

    // Only the name contains the word.
    val isMutableStateFlowReady = true

    // Only a string contains the word.
    val label = "MutableStateFlow"

    // Locals in an init block are not exposed.
    init {
        val local = MutableStateFlow(2)
        local.value = 3
    }

    // Locals in an initializer lambda are not exposed.
    val computed = run {
        val inner = MutableStateFlow(4)
        inner.value
    }

    // Members of an anonymous object are not part of the class API, and the
    // property itself is typed as the anonymous object's supertype.
    val listener = object : Listener {
        val member = MutableStateFlow(5)
        override fun onEvent() {
            member.value = 6
        }
    }
}

class SecondaryConstructed {
    // Go reports this because tree-sitter parses a secondary constructor's body
    // as a block, not a function_body; FIR is correct because a constructor
    // local is not part of the class API.
    constructor() {
        val local = MutableStateFlow(7)
        local.value = 8
    }
}

enum class Mode {
    A {
        // Go reports this because an enum entry body is a class body in
        // tree-sitter; FIR is correct because the entry body is an anonymous
        // object whose members are not visible through Mode.
        val state = MutableStateFlow(9)
    },
    B,
}

// Go reports this because the class name contains the text "MutableStateFlow";
// FIR is correct because the type is not a MutableStateFlow subtype.
class MutableStateFlowSettings(val retries: Int)

class SettingsHolder {
    val settings = MutableStateFlowSettings(3)
}
