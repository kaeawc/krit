// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 18, 20, 35
// Positive: a property whose type is a subtype of MutableStateFlow still hands
// callers a writable state holder, so it is reported like MutableStateFlow itself.
package test.subtype

import kotlinx.coroutines.flow.MutableStateFlow

class SavedMutableStateFlow<T>(initial: T) : MutableStateFlow<T> by MutableStateFlow(initial)

interface CounterFlow : MutableStateFlow<Int>

class Box : MutableStateFlow<Int> by MutableStateFlow(0)

class ViewModel(private val counter: CounterFlow) {
    <!StateFlowMutableLeak!>val<!> saved = SavedMutableStateFlow(0)

    <!StateFlowMutableLeak!>val<!> savedTyped: SavedMutableStateFlow<Int> = SavedMutableStateFlow(1)

    <!StateFlowMutableLeak!>val<!> savedList: List<SavedMutableStateFlow<Int>> = emptyList()

    // Deliberate improvements: Go matches the text "MutableStateFlow" and misses
    // a subtype whose name does not contain it (an interface or a class that
    // extends MutableStateFlow); FIR resolves the supertype.
    <!StateFlowMutableLeak!>val<!> exposedCounter: CounterFlow = counter

    <!StateFlowMutableLeak!>val<!> box = Box()

    // A private subtype-typed property is hidden like any other.
    private val hiddenSaved = SavedMutableStateFlow(2)
}

// A type parameter bounded by MutableStateFlow exposes it too (Go reports this
// through the bound's text).
<!StateFlowMutableLeak!>val<!> <T : MutableStateFlow<Int>> T.self: T
    get() = this
