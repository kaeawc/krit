package p.lambdas

import androidx.compose.ui.input.pointer.PointerInputScope
import androidx.compose.ui.input.pointer.pointerInput

suspend fun PointerInputScope.inRun() {
    run { awaitPointerEventScope { awaitPointerEvent() } }
}

suspend fun PointerInputScope.inForEach() {
    listOf(1).forEach { awaitPointerEventScope { awaitPointerEvent() } }
}

suspend fun PointerInputScope.ifBody(c: Boolean) = if (c) awaitPointerEventScope { awaitPointerEvent() } else null

suspend fun PointerInputScope.unitBody() = awaitPointerEventScope { }

suspend fun PointerInputScope.labeledReturn() {
    run {
        return@run awaitPointerEventScope { awaitPointerEvent() }
    }
}

class Holder(val scope: PointerInputScope) {
    val events = suspend { scope.awaitPointerEventScope { awaitPointerEvent() } }
}
