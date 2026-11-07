// rules: UnusedPrivateProperty, StateFlowMutableLeak, NonBooleanPropertyPrefixedWithIs
package compat

import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow

class Holder {
    // The explicit backing field keeps the mutable flow private, so the leak rule stays silent.
    val state: StateFlow<Int>
        field = MutableStateFlow(0)

    val events: SharedFlow<Int>
        field = MutableSharedFlow<Int>()

    val typedField: StateFlow<Int> field: MutableStateFlow<Int> = MutableStateFlow(0)

    val names: List<String>
        field = mutableListOf<String>()

    // A real leak: the declared type is mutable.
    val leaked = MutableStateFlow(0) // expect: StateFlowMutableLeak

    private val unusedBacked: Int // expect: UnusedPrivateProperty
        field = 5

    private val usedBacked: Int
        field = 7

    val isReady: Boolean
        field = true

    fun bump() {
        (state as MutableStateFlow).value = names.size
        println(usedBacked)
    }
}
