// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import androidx.annotation.MainThread
import javax.inject.Singleton
import java.util.concurrent.atomic.AtomicInteger

@Singleton
interface Contract { var value: Int }

@Singleton
abstract class Base { abstract var value: Int }

// Constructor properties are outside Go's class-body property scan.
@Singleton
class ConstructorState(var value: Int)

@Singleton
class ReadOnly {
    val entries = listOf<String>()
    val keys = setOf<String>()
    val mappings = mapOf<String, String>()
    val wrapped = java.util.Collections.unmodifiableList(listOf("value"))
}

@Singleton
class Protected {
    @Volatile private var visible = 0
    var counter = AtomicInteger(0)
    @get:Synchronized @set:Synchronized var guarded = 0
    private var value = 0
    fun updateProtection() { visible++; counter.incrementAndGet(); guarded++ }
    @Synchronized fun bump() { value++ }
    @Synchronized fun read() = value
}

@Singleton
class Locked {
    private var value = 0
    fun bump() { synchronized(this) { value++ } }
    fun read() = synchronized(this) { value }
}

@Singleton
class PrivateHelper {
    private var value = 0
    private fun update() { value++ }
    @Synchronized fun bump() { update() }
}

@Singleton
class ThreadAnnotated {
    private var value = 0
    @MainThread fun bump() { value++ }
    @MainThread fun read() = value
}

@MainThread @Singleton
class Confined {
    var value = 0
    fun bump() { value++ }
}

@Singleton
class DispatcherConfined(private val dispatcher: kotlinx.coroutines.MainCoroutineDispatcher) {
    var value = 0
    fun bump() { value++ }
}

@Singleton
class ExternalAssignment {
    @javax.inject.Inject lateinit var value: String
    fun read() = value
}

class Scratch {
    var value = 0
    val entries = mutableListOf<String>()
}

// Go dispatches class declarations, not object declarations.
@Singleton
object ObjectState { var value = 0 }

class CompanionState {
    @Singleton
    companion object { var value = 0 }
}
