// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 36
// Explicit backing fields (stable in Kotlin 2.4) keep the mutable flow in the field,
// so the property's own type is what callers see and nothing leaks.
package test

import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow

class ExplicitBackingFields {
    val inferred: StateFlow<Int>
        field = MutableStateFlow(0)

    val typed: StateFlow<Int>
        field: MutableStateFlow<Int> = MutableStateFlow(0)

    val singleLine: StateFlow<Int> field = MutableStateFlow(0)

    val singleLineTyped: StateFlow<Int> field: MutableStateFlow<Int> = MutableStateFlow(0)

    val shared: SharedFlow<Int>
        field = MutableSharedFlow<Int>()

    fun write() {
        inferred.value = 1
        typed.value = 2
        singleLine.value = 3
        singleLineTyped.value = 4
        shared.tryEmit(5)
    }
}

class StillLeaks {
    <!StateFlowMutableLeak!>val<!> leaked: MutableStateFlow<Int>
        field = MutableStateFlow(0)
}
