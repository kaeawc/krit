package p

import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.PointerEvent
import androidx.compose.ui.input.pointer.PointerInputScope
import androidx.compose.ui.input.pointer.pointerInput
import kotlinx.coroutines.coroutineScope

fun Modifier.twoScopes(): Modifier = pointerInput(Unit) {
    awaitPointerEventScope { awaitPointerEvent() }
    awaitPointerEventScope { awaitPointerEvent() }
}

fun Modifier.oneScope(): Modifier = pointerInput(Unit) {
    awaitPointerEventScope {
        while (true) {
            awaitPointerEvent()
        }
    }
}

fun Modifier.loop(): Modifier = pointerInput(Unit) {
    while (true) {
        awaitPointerEventScope { awaitPointerEvent() }
    }
}

fun Modifier.separateModifiers(): Modifier = pointerInput(Unit) {
    awaitPointerEventScope { awaitPointerEvent() }
}.pointerInput(Unit) {
    awaitPointerEventScope { awaitPointerEvent() }
}

suspend fun PointerInputScope.helper() {
    awaitPointerEventScope { awaitPointerEvent() }
    awaitPointerEventScope { awaitPointerEvent() }
}

suspend fun PointerInputScope.single() {
    awaitPointerEventScope { awaitPointerEvent() }
}

suspend fun PointerInputScope.returned(): PointerEvent = awaitPointerEventScope { awaitPointerEvent() }

suspend fun PointerInputScope.returnedBlock(): PointerEvent {
    return awaitPointerEventScope { awaitPointerEvent() }
}

suspend fun PointerInputScope.assigned() {
    val event = awaitPointerEventScope { awaitPointerEvent() }
    println(event)
}

fun Modifier.assignedInPointerInput(): Modifier = pointerInput(Unit) {
    val event = awaitPointerEventScope { awaitPointerEvent() }
    println(event)
}

fun Modifier.lastExpression(): Modifier = pointerInput(Unit) {
    awaitPointerEventScope { awaitPointerEvent() }
}

suspend fun PointerInputScope.inCoroutineScope() = coroutineScope {
    awaitPointerEventScope { awaitPointerEvent() }
}

val handler: suspend PointerInputScope.() -> Unit = {
    awaitPointerEventScope { awaitPointerEvent() }
}

val untypedHandler = suspend { }

suspend fun PointerInputScope.parenthesized(): PointerEvent = (awaitPointerEventScope { awaitPointerEvent() })

suspend fun PointerInputScope.var2() {
    var event: PointerEvent? = null
    event = awaitPointerEventScope { awaitPointerEvent() }
}

suspend fun PointerInputScope.lambdaReturn() {
    val f: suspend () -> PointerEvent = { awaitPointerEventScope { awaitPointerEvent() } }
    f()
}

fun Modifier.gesture(): Modifier = pointerInput(Unit) {
    awaitEachGesture {
        awaitPointerEvent()
    }
    awaitPointerEventScope { awaitPointerEvent() }
}
