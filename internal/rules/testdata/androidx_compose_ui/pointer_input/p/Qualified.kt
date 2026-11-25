package p.qualified

import androidx.compose.ui.input.pointer.PointerInputScope

class Holder(val scope: PointerInputScope) {
    val events = suspend { scope.awaitPointerEventScope { awaitPointerEvent() } }
    val events2: suspend () -> Unit = { scope.awaitPointerEventScope { awaitPointerEvent() } }
    suspend fun member() = scope.awaitPointerEventScope { awaitPointerEvent() }
    suspend fun memberRun() { run { scope.awaitPointerEventScope { awaitPointerEvent() } } }
}

suspend fun PointerInputScope.localSuspend() {
    val f = suspend { awaitPointerEventScope { awaitPointerEvent() } }
    f()
}

suspend fun PointerInputScope.qualifiedThis() {
    run { this.awaitPointerEventScope { awaitPointerEvent() } }
}

suspend fun PointerInputScope.nested() {
    run {
        run { awaitPointerEventScope { awaitPointerEvent() } }
    }
}
