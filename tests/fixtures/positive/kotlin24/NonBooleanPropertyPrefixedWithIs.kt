package fixtures.positive.kotlin24

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

class ExplicitBackingFieldNaming {
    // The exposed type is StateFlow<Boolean>, not Boolean: the `is` prefix is
    // misleading even though the explicit backing field holds a Boolean flow.
    val isLoading: StateFlow<Boolean>
        field = MutableStateFlow(false)
}
