package r

import androidx.compose.animation.core.Animatable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.text.input.TextFieldState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.annotation.RememberInComposition
import androidx.compose.runtime.movableContentOf
import androidx.compose.runtime.remember
import androidx.compose.ui.focus.FocusRequester

class Foo @RememberInComposition constructor()

@RememberInComposition
fun test(): Int = 1

interface Bar {
    @RememberInComposition
    fun create(): Int
    @get:RememberInComposition
    val value: Int
}

class BarImpl : Bar {
    override fun create(): Int = 1
    override val value: Int get() = 1
}

object Obj {
    @get:RememberInComposition
    val value: Int get() = 1
}

inline fun runBlock(block: () -> Unit) = block()
inline fun runCross(crossinline block: () -> Unit) = block()

@Composable
fun Positives(bar: Bar, impl: BarImpl) {
    Foo()
    test()
    bar.create()
    bar.value
    impl.create()
    impl.value
    Obj.value
    MutableInteractionSource()
    FocusRequester()
    Animatable(0f)
    TextFieldState()
    movableContentOf { }
    val x = object { val y = Foo() }
    1.let { test() }
    runBlock { test() }
}

@Composable
fun Negatives(bar: Bar) {
    remember { Foo() }
    remember(bar) { bar.create() }
    remember { MutableInteractionSource() }
    runCross { test() }
    val onClick = { FocusRequester() }
}

fun notComposable() {
    Foo()
    MutableInteractionSource()
}
