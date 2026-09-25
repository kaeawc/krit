// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: hidden, overriding, constructor, and function-local MutableStateFlow
// properties, none of which the Go rule reports.
package test

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

class ViewModel {
    private val _state = MutableStateFlow(0)
    val state: StateFlow<Int> = _state.asStateFlow()

    protected val protectedState = MutableStateFlow(1)

    internal val debugState = MutableStateFlow(2)

    private val _items = MutableStateFlow(emptyList<String>())
    val items: StateFlow<List<String>> get() = _items

    fun update() {
        val local = MutableStateFlow(3)
        local.value = 4
        class LocalHolder {
            val member = MutableStateFlow(5)
        }
        LocalHolder().member.value = 6
        val handler = {
            val inLambda = MutableStateFlow(7)
            inLambda.value
        }
        handler()
    }
}

abstract class BaseViewModel {
    protected abstract val exposedState: MutableStateFlow<Int>
}

class RealViewModel : BaseViewModel() {
    override val exposedState = MutableStateFlow(0)
}

interface Contract {
    val state: StateFlow<Int>
}

class ContractImpl : Contract {
    override val state: MutableStateFlow<Int> = MutableStateFlow(0)
}

// Primary-constructor parameters are not property declarations in Go.
class Injected(val state: MutableStateFlow<Int>)

private val filePrivate = MutableStateFlow(0)

internal val moduleInternal = MutableStateFlow(0)

fun topLevelFunction() {
    val local = MutableStateFlow(false)
    local.value = true
}

// The read-only flow type is what the property exposes.
val readOnlyTopLevel: StateFlow<Int> = filePrivate
