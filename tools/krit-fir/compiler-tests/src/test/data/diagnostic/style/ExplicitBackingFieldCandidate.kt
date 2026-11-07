// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 26, 30, 33, 36, 39, 42, 45, 49, 51, 54, 57, 60, 65, 78, 88, 93, 99, 104, 108, 111, 114, 117, 120, 123, 126
// Positive: a private mutable property that only exposes a read-only supertype
// of itself. Reported on the private property, where the edit removes code.
package test

import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.lifecycle.LiveData
import androidx.lifecycle.MutableLiveData
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.channels.ReceiveChannel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

class Flows {
    <!ExplicitBackingFieldCandidate!>private<!> val _byGetter = MutableStateFlow(0)
    val byGetter: StateFlow<Int> get() = _byGetter

    <!ExplicitBackingFieldCandidate!>private<!> val _byBlockGetter = MutableStateFlow(0)
    val byBlockGetter: StateFlow<Int>
        get() { return _byBlockGetter }

    <!ExplicitBackingFieldCandidate!>private<!> val _byWrapper = MutableStateFlow(0)
    val byWrapper: StateFlow<Int> = _byWrapper.asStateFlow()

    <!ExplicitBackingFieldCandidate!>private<!> val _inferred = MutableStateFlow(0)
    val inferred = _inferred.asStateFlow()

    <!ExplicitBackingFieldCandidate!>private<!> val _shared = MutableSharedFlow<Int>()
    val shared: SharedFlow<Int> get() = _shared.asSharedFlow()

    <!ExplicitBackingFieldCandidate!>private<!> val _stateAsShared = MutableStateFlow(0)
    val stateAsShared: SharedFlow<Int> get() = _stateAsShared

    <!ExplicitBackingFieldCandidate!>private<!> val _asFlow = MutableStateFlow(0)
    val asFlow: Flow<Int> = _asFlow

    <!ExplicitBackingFieldCandidate!>private<!> val _declared: MutableStateFlow<Int?> = MutableStateFlow(null)
    val declared: StateFlow<Int?> get() = _declared

    val exposedFirst: StateFlow<Int> get() = _exposedFirst
    <!ExplicitBackingFieldCandidate!>private<!> val _exposedFirst = MutableStateFlow(0)

    <!ExplicitBackingFieldCandidate!>private<!> val _thisQualified = MutableStateFlow(0)
    val thisQualified: StateFlow<Int> get() = this._thisQualified

    <!ExplicitBackingFieldCandidate!>private<!> val backing = MutableStateFlow(0)
    val named: StateFlow<Int> get() = backing

    <!ExplicitBackingFieldCandidate!>private<!> val _internal = MutableStateFlow(0)
    internal val internal: StateFlow<Int> get() = _internal

    <!ExplicitBackingFieldCandidate!>private<!> val _annotatedPublic = MutableStateFlow(0)
    @Suppress("unused") val annotated: StateFlow<Int> get() = _annotatedPublic

    // The backing property has other uses: after the rewrite they go through the
    // smart cast, so the rule still reports it.
    <!ExplicitBackingFieldCandidate!>private<!> val _busy = MutableStateFlow(0)
    val busy: StateFlow<Int> get() = _busy

    fun work() {
        _busy.value = 1
        _busy.update { it + 1 }
        println(_busy)
        consume(_busy)
    }

    private fun consume(flow: MutableStateFlow<Int>) = flow.tryEmit(0)

    /** Documented backing property: Go reports on the declaration's first line. */
    <!ExplicitBackingFieldCandidate!>private<!> val _documented = MutableStateFlow(0)
    val documented: StateFlow<Int> get() = _documented
}

interface HasState {
    val state: StateFlow<Int>
}

// An override is final in a class that cannot be subclassed.
class Overrides : HasState {
    <!ExplicitBackingFieldCandidate!>private<!> val _state = MutableStateFlow(0)
    override val state: StateFlow<Int> get() = _state
}

object Singleton {
    <!ExplicitBackingFieldCandidate!>private<!> val _state = MutableStateFlow(0)
    val state: StateFlow<Int> get() = _state
}

class WithCompanion {
    companion object {
        <!ExplicitBackingFieldCandidate!>private<!> val _state = MutableStateFlow(0)
        val state: StateFlow<Int> get() = _state
    }
}

<!ExplicitBackingFieldCandidate!>private<!> val _topLevel = MutableStateFlow(0)
val topLevel: StateFlow<Int> get() = _topLevel

class Others {
    <!ExplicitBackingFieldCandidate!>private<!> val _live = MutableLiveData<String>()
    val live: LiveData<String> get() = _live

    <!ExplicitBackingFieldCandidate!>private<!> val _list = mutableListOf<Int>()
    val list: List<Int> get() = _list

    <!ExplicitBackingFieldCandidate!>private<!> val _set: MutableSet<Int> = mutableSetOf()
    val set: Set<Int> = _set

    <!ExplicitBackingFieldCandidate!>private<!> val _map = mutableMapOf<String, Int>()
    val map: Map<String, Int> get() = _map

    <!ExplicitBackingFieldCandidate!>private<!> val _arrayList = ArrayList<Int>()
    val arrayList: Collection<Int> get() = _arrayList

    <!ExplicitBackingFieldCandidate!>private<!> val _compose = mutableStateOf(0)
    val compose: State<Int> get() = _compose

    <!ExplicitBackingFieldCandidate!>private<!> val _channel = Channel<Int>()
    val channel: ReceiveChannel<Int> get() = _channel
}
