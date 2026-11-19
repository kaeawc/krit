// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 15, 21, 24, 26, 31, 37, 39, 43, 48, 52, 56, 58, 62, 72, 76
// Positive: public, non-override properties whose type exposes MutableStateFlow.
// Each is reported on the property name, the line the Go rule reports, so an
// annotation or modifier on its own line does not move it.
package test

import kotlinx.coroutines.flow.MutableStateFlow

typealias Counter = MutableStateFlow<Int>

class ViewModel {
    val <!StateFlowMutableLeak!>inferred<!> = MutableStateFlow(0)

    val <!StateFlowMutableLeak!>typed<!>: MutableStateFlow<Int> = MutableStateFlow(1)

    /**
     * KDoc is not part of the reported line.
     */
    @Volatile
    var <!StateFlowMutableLeak!>annotated<!> = MutableStateFlow(2)

    public
    val <!StateFlowMutableLeak!>explicitPublic<!> = MutableStateFlow(3)

    var <!StateFlowMutableLeak!>privateSetter<!> = MutableStateFlow(4)
        private set

    val <!StateFlowMutableLeak!>delegated<!> by lazy { MutableStateFlow(5) }

    val <!StateFlowMutableLeak!>getterBacked<!>: MutableStateFlow<Int>
        get() {
            val local = MutableStateFlow(6)
            return local
        }

    val <!StateFlowMutableLeak!>nullable<!>: MutableStateFlow<String>? = null

    val <!StateFlowMutableLeak!>inCollection<!>: List<MutableStateFlow<Int>> = emptyList()

    val <!StateFlowMutableLeak!>inMap<!> = mutableMapOf<String, MutableStateFlow<Int>>()

    val <!StateFlowMutableLeak!>qualified<!> = kotlinx.coroutines.flow.MutableStateFlow(7)

    val <!StateFlowMutableLeak!>aliased<!>: Counter = MutableStateFlow(8)

    companion object {
        val <!StateFlowMutableLeak!>shared<!> = MutableStateFlow(9)
    }

    object Nested {
        val <!StateFlowMutableLeak!>nested<!> = MutableStateFlow(10)
    }
}

val <!StateFlowMutableLeak!>topLevel<!> = MutableStateFlow(11)

val String.<!StateFlowMutableLeak!>extension<!>: MutableStateFlow<String>
    get() = MutableStateFlow(this)

object Store {
    val <!StateFlowMutableLeak!>state<!> = MutableStateFlow(12)
}

// A public property of a private class has private effective visibility.
private class HiddenHolder {
    val state = MutableStateFlow(13)
}

// The mutable contract is reported; the override that implements it is not.
interface Contract {
    val <!StateFlowMutableLeak!>state<!>: MutableStateFlow<Int>
}

abstract class Base {
    abstract val <!StateFlowMutableLeak!>exposed<!>: MutableStateFlow<Int>
}

// A factory result is still a MutableStateFlow even when the source never
// spells the type (Go's substring match misses this one).
fun createCounter(): MutableStateFlow<Int> = MutableStateFlow(0)

class Holder {
    val <!StateFlowMutableLeak!>fromFactory<!> = createCounter()
}
