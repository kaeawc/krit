package style

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

class CounterViewModel {
    // Already an explicit backing field.
    val count: StateFlow<Int>
        field = MutableStateFlow(0)

    // The public property computes a value, so the private one cannot be its field.
    private val _total = MutableStateFlow(0)
    val total: StateFlow<Int> get() = _total.asStateFlow().also { println(it) }

    // The mutable flow is used by two properties.
    private val _shared = MutableStateFlow(0)
    val first: StateFlow<Int> = _shared.asStateFlow()
    val second: StateFlow<Int> = _shared.asStateFlow()

    // The private property is not exposed at all.
    private val _internalOnly = MutableStateFlow(0)

    fun increment() {
        count.value += 1
        _total.value += 1
        _internalOnly.value += 1
        _shared.value += 1
    }
}

open class OpenViewModel {
    private val _open = MutableStateFlow(0)
    open val open: StateFlow<Int> get() = _open
}
