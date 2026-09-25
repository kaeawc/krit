// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: public, non-override properties whose type exposes MutableStateFlow.
// Each is reported on the property's first line (modifier list, else val/var),
// the line the Go rule reports.
package test

import kotlinx.coroutines.flow.MutableStateFlow

typealias Counter = MutableStateFlow<Int>

class ViewModel {
    <!StateFlowMutableLeak!>val<!> inferred = MutableStateFlow(0)

    <!StateFlowMutableLeak!>val<!> typed: MutableStateFlow<Int> = MutableStateFlow(1)

    /**
     * KDoc is not part of the reported line.
     */
    <!StateFlowMutableLeak!>@Volatile<!>
    var annotated = MutableStateFlow(2)

    <!StateFlowMutableLeak!>public<!>
    val explicitPublic = MutableStateFlow(3)

    <!StateFlowMutableLeak!>var<!> privateSetter = MutableStateFlow(4)
        private set

    <!StateFlowMutableLeak!>val<!> delegated by lazy { MutableStateFlow(5) }

    <!StateFlowMutableLeak!>val<!> getterBacked: MutableStateFlow<Int>
        get() {
            val local = MutableStateFlow(6)
            return local
        }

    <!StateFlowMutableLeak!>val<!> nullable: MutableStateFlow<String>? = null

    <!StateFlowMutableLeak!>val<!> inCollection: List<MutableStateFlow<Int>> = emptyList()

    <!StateFlowMutableLeak!>val<!> inMap = mutableMapOf<String, MutableStateFlow<Int>>()

    <!StateFlowMutableLeak!>val<!> qualified = kotlinx.coroutines.flow.MutableStateFlow(7)

    <!StateFlowMutableLeak!>val<!> aliased: Counter = MutableStateFlow(8)

    companion object {
        <!StateFlowMutableLeak!>val<!> shared = MutableStateFlow(9)
    }

    object Nested {
        <!StateFlowMutableLeak!>val<!> nested = MutableStateFlow(10)
    }
}

<!StateFlowMutableLeak!>val<!> topLevel = MutableStateFlow(11)

<!StateFlowMutableLeak!>val<!> String.extension: MutableStateFlow<String>
    get() = MutableStateFlow(this)

object Store {
    <!StateFlowMutableLeak!>val<!> state = MutableStateFlow(12)
}

// Go checks only the property's own modifiers, so a public property of a
// private class is still reported.
private class HiddenHolder {
    <!StateFlowMutableLeak!>val<!> state = MutableStateFlow(13)
}

// The mutable contract is reported; the override that implements it is not.
interface Contract {
    <!StateFlowMutableLeak!>val<!> state: MutableStateFlow<Int>
}

abstract class Base {
    <!StateFlowMutableLeak!>abstract<!> val exposed: MutableStateFlow<Int>
}

// A factory result is still a MutableStateFlow even when the source never
// spells the type (Go's substring match misses this one).
fun createCounter(): MutableStateFlow<Int> = MutableStateFlow(0)

class Holder {
    <!StateFlowMutableLeak!>val<!> fromFactory = createCounter()
}
