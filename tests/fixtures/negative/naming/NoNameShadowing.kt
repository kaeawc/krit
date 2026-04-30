package com.example.naming

fun example() {
    val x = 1
    val items = listOf(1, 2, 3)
    items.forEach { y ->
        println(y)
    }

    val name = "outer"
    items.map { item ->
        item.toString()
    }
    items.map { name ->
        name.toString()
    }

    // Underscore names should not be flagged
    val _ = 1
    items.forEach { _ ->
        println("ignored")
    }
}

// Class member function params should not shadow constructor params
// (accessible via this.name)
class Foo(val a: Int) {
    fun foo(a: Int) {}
}

// Nested non-inner classes have separate scope
class Bar(val x: Int) {
    class Nested {
        fun bar(x: Int) {}
    }
}

// Inner classes: member function params don't shadow outer class params
// (accessible via this@Outer.name)
class Outer(val y: Int) {
    inner class Inner {
        fun test(y: Int) {}
    }
}

// Companion object has separate scope
class WithCompanion(val z: Int) {
    companion object {
        fun comp(z: Int) {}
    }
}

// Object declaration has separate scope
class WithObject(val w: Int) {
    object Nested {
        fun obj(w: Int) {}
    }
}

// Secondary constructor params don't shadow primary constructor params
class SecCtor(context: String) {
    constructor(context: String, extra: Int) : this(context)
}

class ConstructorBackedProperty(allocatedNames: Set<String>) {
    private val allocatedNames =
        mutableMapOf<String, Unit>().apply {
            for (allocated in allocatedNames) {
                put(allocated, Unit)
            }
        }
}

class ContextWrapper(
    private val options: Options,
    expectActualTracker: ExpectActualTracker,
) {
    val expectActualTracker: ExpectActualTracker =
        if (options.reportsEnabled) {
            RecordingExpectActualTracker(this, expectActualTracker)
        } else {
            expectActualTracker
        }
}

class Options(val reportsEnabled: Boolean)
open class ExpectActualTracker
class RecordingExpectActualTracker(owner: Any, delegate: ExpectActualTracker) : ExpectActualTracker()

// Same variable name in sibling if/else branches should NOT be flagged
fun siblingBranches(value: Int) {
    if (value > 0) {
        val result = "positive"
        println(result)
    } else if (value < 0) {
        val result = "negative"
        println(result)
    } else {
        val result = "zero"
        println(result)
    }
}

// Same variable in sibling when branches should NOT be flagged
fun whenBranches(value: Int) {
    when {
        value > 0 -> {
            val label = "positive"
            println(label)
        }
        value < 0 -> {
            val label = "negative"
            println(label)
        }
    }
}

// Typealias parameter labels should NOT shadow lambda params
typealias RequestPredicate = (request: String) -> Boolean

fun useTypealias() {
    val predicate: RequestPredicate = { request -> request.isNotEmpty() }
    println(predicate)
}
