// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: shapes where an explicit backing field does not replace the private
// property, or does not exist.
package test

import androidx.lifecycle.LiveData
import androidx.lifecycle.MutableLiveData
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

interface HasState {
    val state: StateFlow<Int>
}

class Negatives : HasState {
    // Already an explicit backing field.
    val explicit: StateFlow<Int>
        field = MutableStateFlow(0)

    // The public property does more than expose the private one.
    private val _computed = MutableStateFlow(0)
    val computed: StateFlow<Int> get() = _computed.asStateFlow().also { println(it) }

    // A copy is a different object.
    private val _copy = mutableListOf(1)
    val copy: List<Int> get() = _copy.toList()

    // Two properties expose the same mutable property.
    private val _twice = MutableStateFlow(0)
    val first: StateFlow<Int> = _twice.asStateFlow()
    val second: StateFlow<Int> = _twice.asStateFlow()

    // Not exposed at all.
    private val _hidden = MutableStateFlow(0)

    // The mutable property is not private.
    internal val _internalBacking = MutableStateFlow(0)
    val viaInternal: StateFlow<Int> get() = _internalBacking

    // A var cannot become a backing field, and a var public property cannot have one.
    private var _var = MutableStateFlow(0)
    val viaVar: StateFlow<Int> get() = _var
    private val _forVar = MutableStateFlow(0)
    var publicVar: StateFlow<Int> = _forVar

    // The public type is the mutable type, or the property is private.
    private val _mutableOut = MutableStateFlow(0)
    val mutableOut: MutableStateFlow<Int> get() = _mutableOut
    private val _privateOut = MutableStateFlow(0)
    private val privateOut: StateFlow<Int> get() = _privateOut

    // A delegated property has no field of its own.
    private val _delegated = MutableStateFlow(0)
    val delegated: StateFlow<Int> by lazy { _delegated }

    // The private property has an annotation.
    @Suppress("unused")
    private val _annotated = MutableStateFlow(0)
    val annotated: StateFlow<Int> get() = _annotated

    // Not a mutable family type.
    private val _plain = Plain()
    val plain: Marker get() = _plain

    override val state: StateFlow<Int> = MutableStateFlow(0)

    // A property with a receiver has no backing field.
    private val _receiver = MutableStateFlow(0)
    val Int.withReceiver: StateFlow<Int> get() = _receiver

    // A mutable collection exposed with a type that is not its read-only counterpart.
    private val _list = mutableListOf(1)
    val asAny: Any get() = _list

    // A nullable exposure keeps the explicit nullability of the public type.
    private val _liveNullable = MutableLiveData<String>()
    val liveNullable: LiveData<String>? get() = _liveNullable
}

interface Marker
class Plain : Marker

// An override in an open class may be overridden again.
open class Open : HasState {
    private val _state = MutableStateFlow(0)
    override val state: StateFlow<Int> get() = _state

    private val _openProp = MutableStateFlow(0)
    open val openProp: StateFlow<Int> get() = _openProp
}

// Already rewritten: an override and a top-level property with explicit backing fields.
class ExplicitOverride : HasState {
    override val state: StateFlow<Int>
        field = MutableStateFlow(0)

    fun set(value: Int) {
        state.value = value
    }
}

val topLevelExplicit: StateFlow<Int>
    field = MutableStateFlow(0)

fun setTopLevel(value: Int) {
    topLevelExplicit.value = value
}

// A primary-constructor property is a parameter, not a declaration in the body.
class FromConstructor(private val events: MutableStateFlow<Int>) {
    val exposed: StateFlow<Int> = events
}
