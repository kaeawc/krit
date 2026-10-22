package fixtures.positive.kotlin24

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

class ExplicitBackingFieldViewModel {
    // Exposed type is the read-only StateFlow: not a leak.
    val state: StateFlow<Int>
        field = MutableStateFlow(0)

    // The exposed type is the mutable flow itself: the explicit backing
    // field does not hide it.
    val leaked: MutableStateFlow<Int>
        field = MutableStateFlow(0)
}
