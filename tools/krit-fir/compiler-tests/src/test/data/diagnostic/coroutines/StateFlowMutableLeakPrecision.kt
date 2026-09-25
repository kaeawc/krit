// RENDER_DIAGNOSTICS_FULL_TEXT
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
