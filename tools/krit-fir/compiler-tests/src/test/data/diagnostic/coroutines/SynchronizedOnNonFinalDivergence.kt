// RENDER_DIAGNOSTICS_FULL_TEXT
// Where FIR resolution and the Go rule's name-based lookup disagree. Go
// reports a bare lock name when any `var` of that name is declared inside the
// nearest enclosing class or object; FIR reports when the name resolves to a
// Kotlin `var`, wherever it is declared.
package test

var topLevelMutable = Any()

class ParameterShadow {
    private var lock = Any()

    // Go reports this because the class declares `var lock`; FIR is correct
    // to drop it because the lock is the final parameter.
    fun f(lock: Any) {
        synchronized(lock) { }
    }

    // Go reports this because the class declares `var lock`; FIR is correct
    // to drop it because the lock is the final local `val`.
    fun g() {
        val lock = Any()
        synchronized(lock) { }
    }

    // Go reports this because the class declares `var lock`; FIR is correct
    // to drop it because the lock is the final lambda parameter.
    fun h() {
        listOf(Any()).forEach { lock -> synchronized(lock) { } }
    }
}

class NameCollision {
    private val guard = Any()

    fun other() {
        var guard = 0
        guard++
        println(guard)
    }

    // Go reports this because other() declares a local `var guard`; FIR is
    // correct to drop it because the lock is the final member `val guard`.
    fun f() {
        synchronized(guard) { }
    }
}

class NestedCollision {
    private val gate = Any()

    class Nested {
        var gate = Any()
    }

    // Go reports this because the nested class declares `var gate`; FIR is
    // correct to drop it because the lock is the outer final `val gate`.
    fun f() {
        synchronized(gate) { }
    }
}

class GoMissesTopLevel {
    // Go misses this because the `var` is declared outside the class; FIR is
    // correct because the lock is the top-level `var topLevelMutable`.
    fun f() {
        <!SynchronizedOnNonFinal!>synchronized(topLevelMutable) { }<!>
    }
}

// Go misses this because a primary-constructor `var` is a class parameter,
// not a property declaration; FIR is correct because the lock is that `var`.
class GoMissesConstructorVar(var ctorLock: Any) {
    fun f() {
        <!SynchronizedOnNonFinal!>synchronized(ctorLock) { }<!>
    }
}

class Outer {
    var outerLock = Any()

    // Go misses this because it searches only the nearest class (Inner); FIR
    // is correct because the lock is Outer's `var outerLock`.
    inner class Inner {
        fun f() {
            <!SynchronizedOnNonFinal!>synchronized(outerLock) { }<!>
        }
    }
}

open class Base {
    var inherited = Any()
}

class Derived : Base() {
    // Go misses this because the `var` is declared in the superclass; FIR is
    // correct because the lock is the inherited `var inherited`.
    fun f() {
        <!SynchronizedOnNonFinal!>synchronized(inherited) { }<!>
    }
}

class Box {
    var content = Any()
}

class ScopeReceiver {
    // Go misses this because ScopeReceiver declares no `var content`; FIR is
    // correct because the lock is the receiver's `var Box.content`.
    fun f(box: Box) {
        with(box) {
            <!SynchronizedOnNonFinal!>synchronized(content) { }<!>
        }
    }
}

class Destructured {
    // Go misses this because it cannot read a name from a destructuring
    // declaration; FIR is correct because the lock is the local `var first`.
    fun f(pair: Pair<Any, Any>) {
        var (first, second) = pair
        <!SynchronizedOnNonFinal!>synchronized(first) { }<!>
        first = second
        second = first
    }
}

// Go misses these because it needs an enclosing class or object; FIR is
// correct because each lock is a `var`.
fun topLevelFunction() {
    <!SynchronizedOnNonFinal!>synchronized(topLevelMutable) { }<!>
    var localLock = Any()
    <!SynchronizedOnNonFinal!>synchronized(localLock) { }<!>
    localLock = Any()
}

// Go reports these because the class declares `var lock`; FIR is correct to
// drop them because in an init block and a property initializer the name
// resolves to the final primary-constructor parameter `lock`.
class InitShadow(lock: Any) {
    var lock = lock

    init {
        synchronized(lock) { }
    }

    val v = synchronized(lock) { 1 }
}

class FinalShadows {
    var lock = Any()
    var item = Any()
    var e = Any()

    // Go reports this because the nearest class is FinalShadows, whose body
    // declares `var lock`; FIR is correct to drop it because the lock is the
    // object expression's final `val lock`.
    fun objectShadow(): Runnable = object : Runnable {
        val lock = Any()

        override fun run() {
            synchronized(lock) { }
        }
    }

    // Go reports these because the class declares `var item`, `var e`, and
    // `var lock`; FIR is correct to drop them because each lock is a final
    // binding: a for-loop variable, a catch parameter, and a `when` subject.
    fun loopCatchWhen(items: List<Any>) {
        for (item in items) {
            synchronized(item) { }
        }
        try {
            println(items)
        } catch (e: Exception) {
            synchronized(e) { }
        }
        when (val lock = items.firstOrNull()) {
            null -> Unit
            else -> synchronized(lock) { }
        }
    }
}

class ExtensionOwner {
    var lock = Any()
}

// Go misses this because an extension function has no enclosing class; FIR is
// correct because the lock is the receiver's `var ExtensionOwner.lock`.
fun ExtensionOwner.ext() {
    <!SynchronizedOnNonFinal!>synchronized(lock) { }<!>
}

class Backticks {
    var lock = Any()
    var `fun` = Any()
    var `plain` = Any()

    fun f() {
        // Go reports this too: the declaration and the lock are both spelled
        // with backticks, and the message names the lock as written.
        <!SynchronizedOnNonFinal!>synchronized(`fun`) { }<!>
        // Go misses these because it compares the names as written, backticks
        // included; FIR is correct because each lock is a `var`. The message
        // names the lock as written, as Go's does.
        <!SynchronizedOnNonFinal!>synchronized(`lock`) { }<!>
        <!SynchronizedOnNonFinal!>synchronized(plain) { }<!>
    }
}
