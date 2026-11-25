package c

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.launchIn
import kotlinx.coroutines.launch

@Composable
fun Direct(scope: CoroutineScope) {
    scope.launch {}
    scope.async {}
    flowOf(Unit).launchIn(scope)
    GlobalScope.launch {}
}

@Composable
fun Remembered() {
    val scope = rememberCoroutineScope()
    scope.launch {}
    rememberCoroutineScope().launch {}
}

val topLevel = @Composable {
    CoroutineScope(kotlinx.coroutines.Dispatchers.Main).launch {}
}

val typed: @Composable () -> Unit = {
    GlobalScope.async {}
}

@Composable
fun Lambdas(scope: CoroutineScope, content: @Composable () -> Unit) {
    LambdaParameter(content = { scope.launch {} })
    LambdaParameter { scope.launch {} }
    Column { scope.launch {} }
    Box(content = { scope.async {} })
    CompositionLocalProvider { scope.launch {} }
    LazyColumn {
        scope.launch {}
        item { scope.launch {} }
    }
    scope.let { it.launch {} }
    run { scope.launch {} }
    inlineHelper { scope.launch {} }
    crossHelper { scope.launch {} }
}

@Composable
fun LambdaParameter(content: @Composable () -> Unit) {}

inline fun inlineHelper(block: () -> Unit) = block()
inline fun crossHelper(crossinline block: () -> Unit) = block()

@Composable
fun NotComposition(scope: CoroutineScope) {
    LaunchedEffect(Unit) { scope.launch {} }
    remember { scope.launch {} }
    val onClick = { scope.launch {} }
    Plain { scope.launch {} }
    Plain(onClick = { scope.async {} })
    fun local() { scope.launch {} }
    val o = object { fun f() { scope.launch {} } }
    val p = object { val q = scope.launch {} }
}

fun Plain(onClick: () -> Unit = {}, block: () -> Unit = {}) {}

fun nonComposable(scope: CoroutineScope) {
    scope.launch {}
    val l = { scope.async {} }
}

@Composable
fun Defaults(scope: CoroutineScope, job: Any = scope.launch {}) {}

class Launcher { fun launch(input: String) {} }

@Composable
fun Lookalike(launcher: Launcher) {
    launcher.launch("x")
}

@Composable
fun Accessor(scope: CoroutineScope) = Unit

val Prop: Int
    @Composable get() {
        GlobalScope.launch {}
        return 1
    }
