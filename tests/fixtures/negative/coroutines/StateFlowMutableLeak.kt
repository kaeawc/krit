package test

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

class VM {
    private val _state = MutableStateFlow(0)
    val state: StateFlow<Int> = _state
    internal val debugState = MutableStateFlow(0)
}

fun testLocalState() {
    val local = MutableStateFlow(false)
    local.value = true
}
