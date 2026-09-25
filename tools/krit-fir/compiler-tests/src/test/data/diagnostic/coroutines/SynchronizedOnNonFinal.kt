// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: synchronized() whose lock is a bare name of a `var` declared in
// the enclosing class or object (a member, or a local in one of its
// functions) should trigger SynchronizedOnNonFinal, as Go reports it.
package test

import kotlin.properties.Delegates

class Worker {
    private var lock = Any()
    var typed: Any = Any()
    lateinit var late: Any
    var observed: Any by Delegates.observable(Any()) { _, _, _ -> }
    var withAccessors: Any = Any()
        get() = field
        set(value) {
            field = value
        }
    var nullable: Any? = null

    fun members() {
        <!SynchronizedOnNonFinal!>synchronized(lock) { }<!>
        <!SynchronizedOnNonFinal!>synchronized(typed) { }<!>
        <!SynchronizedOnNonFinal!>synchronized(late) { }<!>
        <!SynchronizedOnNonFinal!>synchronized(observed) { }<!>
        <!SynchronizedOnNonFinal!>synchronized(withAccessors) { }<!>
        <!SynchronizedOnNonFinal!>kotlin.synchronized(lock) { }<!>
        <!SynchronizedOnNonFinal!>synchronized(lock, { })<!>
        val result = <!SynchronizedOnNonFinal!>synchronized(lock) { 1 }<!>
        println(result)
    }

    fun multiLine() {
        <!SynchronizedOnNonFinal!>synchronized<!>(
            lock,
        ) {
        }
    }

    fun smartCast() {
        val current = nullable
        if (current != null) {
            synchronized(current) { }
        }
        var local: Any? = null
        local = Any()
        // A stable local var smart-cast to Any is still the var.
        <!SynchronizedOnNonFinal!>synchronized(local) { }<!>
    }

    fun localVar() {
        var localLock = Any()
        <!SynchronizedOnNonFinal!>synchronized(localLock) { }<!>
        localLock = Any()
    }

    fun nestedScopes() {
        run {
            <!SynchronizedOnNonFinal!>synchronized(lock) { }<!>
        }
        val task = Runnable {
            <!SynchronizedOnNonFinal!>synchronized(lock) { }<!>
        }
        task.run()
        fun local() {
            <!SynchronizedOnNonFinal!>synchronized(lock) { }<!>
        }
        local()
    }

    companion object {
        var shared = Any()

        fun fromCompanion() {
            <!SynchronizedOnNonFinal!>synchronized(shared) { }<!>
        }
    }

    fun companionLock() {
        <!SynchronizedOnNonFinal!>synchronized(shared) { }<!>
    }

    // A member of an object expression: Go's nearest class is Worker, whose
    // body holds the object's `var`.
    fun anonymous(): Runnable = object : Runnable {
        var inner = Any()

        override fun run() {
            <!SynchronizedOnNonFinal!>synchronized(inner) { }<!>
        }
    }
}

object Registry {
    private var monitor = Any()

    fun use() {
        <!SynchronizedOnNonFinal!>synchronized(monitor) { }<!>
    }
}

interface Holder {
    var held: Any

    fun use() {
        <!SynchronizedOnNonFinal!>synchronized(held) { }<!>
    }
}

enum class Mode {
    A;

    var modeLock = Any()

    fun use() {
        <!SynchronizedOnNonFinal!>synchronized(modeLock) { }<!>
    }
}
