// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Lookalikes: a project function with a stdlib builder's name is not a pure stdlib function, so a discarded
// call to it is not reported unless it carries a must-use annotation.
package test

class Pipeline {
    fun map(block: (Int) -> Int): Sequence<Int> = sequenceOf(block(1))
    fun filter(block: (Int) -> Boolean): Sequence<Int> = sequenceOf(1).filter(block)
}

fun Sequence<Int>.mapAll(block: (Int) -> Int): Sequence<Int> = map(block)

fun projectBuilders(pipeline: Pipeline, items: Sequence<Int>) {
    pipeline.map { it + 1 }
    pipeline.filter { it > 1 }
    items.mapAll { it + 1 }
}

// A call that returns a function value through a project receiver is not reported either.
class Handlers {
    fun listener(): (Int) -> Unit = {}
}

fun functionValue(handlers: Handlers) {
    handlers.listener()
}
