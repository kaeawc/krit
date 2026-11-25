package p.lookalike

class Scope {
    fun <T> awaitPointerEventScope(block: () -> T): T = block()
}

fun Scope.twice() {
    awaitPointerEventScope { 1 }
    val x = awaitPointerEventScope { 2 }
}
