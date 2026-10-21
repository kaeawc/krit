package fixtures.negative.kotlin24

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

interface Logger {
    fun log(msg: String)
}

class ExplicitBackingFieldViewModel {
    // Read-only exposed type over a mutable explicit backing field.
    val state: StateFlow<Int>
        field = MutableStateFlow(0)

    // Non-public properties may expose the mutable flow.
    internal val internalState: MutableStateFlow<Int>
        field = MutableStateFlow(1)

    // Accessor `field` is the implicit backing field, not an explicit one.
    var counter: Int = 0
        set(value) {
            field = value
        }

    // A named context parameter does not affect the property checks.
    context(logger: Logger)
    fun snapshot(): Int {
        logger.log("snapshot")
        return internalState.value
    }
}
