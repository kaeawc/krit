// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 27, 34, 42, 85
// An effect keyed only on constants whose body reads a parameter of the enclosing function never restarts
// when that parameter changes. Other keys, bodies that read no parameter, and receiver-only reads are fine.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

fun fetch(value: Any?) {}

class Controller {
    fun start() {}
}

class Holder(val version: Int)

@Composable
fun UnitKey(userId: String) {
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) {
        fetch(userId)
    }
}

@Composable
fun TrueKey(userId: String) {
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(true) {
        fetch(userId)
    }
}

@Composable
fun TwoConstants(userId: String) {
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit, null) {
        fetch(userId)
    }
}

// The message lists every captured parameter in source order, receivers and property reads included.
@Composable
fun ManyParameters(items: List<String>, controller: Controller, state: Holder) {
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) {
        controller.start()
        fetch(items)
        fetch(state.version)
    }
}

@Composable
fun KeyedOnParameter(userId: String) {
    LaunchedEffect(userId) {
        fetch(userId)
    }
}

@Composable
fun KeyedOnOneOfTwo(userId: String) {
    LaunchedEffect(Unit, userId) {
        fetch(userId)
    }
}

@Composable
fun NoParameterRead() {
    LaunchedEffect(Unit) {
        fetch(1)
    }
}

// A parameter read only as the receiver of a method call is not reported on its own.
@Composable
fun ReceiverOnly(controller: Controller) {
    LaunchedEffect(Unit) {
        controller.start()
    }
}

// A local or a lambda parameter with the parameter's name is a different symbol.
@Composable
fun Shadowed(key: Int, items: List<Int>) {
    LaunchedEffect(Unit) {
        val key = 1
        fetch(key)
    }
    <!ComposeLaunchedEffectWithoutKeys!>LaunchedEffect<!>(Unit) {
        fetch(items.size)
        items.forEach { key -> fetch(key) }
    }
}

// Only a trailing lambda is the effect body.
@Composable
fun NamedBlock(userId: String) {
    LaunchedEffect(Unit, block = {
        fetch(userId)
    })
}
