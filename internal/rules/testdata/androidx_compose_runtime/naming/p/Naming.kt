package p

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocal
import androidx.compose.runtime.ProvidableCompositionLocal
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.staticCompositionLocalOf

@Composable
fun button() {}

@Composable
fun Button() {}

@Composable
fun GetInt(): Int = 1

@Composable
fun getInt(): Int = 1

@Composable
fun explicitUnit(): Unit {}

@Composable
fun exprBody() = button()

@Composable
fun <T> GenericValue(): T? = null

@Composable
fun _underscore() {}

@Composable
fun `backticked name`() {}

fun notComposable() {}

interface Iface {
    @Composable
    fun abstractMember()

    @Composable
    fun Content(): String
}

class Impl : Iface {
    @Composable
    override fun abstractMember() {}

    @Composable
    override fun Content(): String = ""
}

object Holder {
    @Composable
    operator fun invoke() {}

    @Composable
    internal fun member() {}
}

@Composable
fun Outer() {
    @Composable
    fun localLower() {}
    localLower()
}

@Composable
fun Lambdas1(foo: Int, text: @Composable () -> Unit) {}

@Composable
fun Lambdas2(content: @Composable () -> Unit, foo: Int) {}

@Composable
fun Lambdas3(text: @Composable () -> Unit, foo: Int) {}

@Composable
fun Lambdas4(a: @Composable () -> Unit, b: @Composable () -> Unit) {}

@Composable
fun Lambdas5(item: @Composable (Int) -> Unit, foo: Int) {}

@Composable
fun Lambdas6(text: () -> Unit) {}

@Composable
fun Lambdas7(text: @Composable (() -> Unit)?, foo: Int = 1) {}

@Composable
fun Lambdas8(text: (@Composable () -> Unit)?) {}

@Composable
fun Lambdas9(body: @Composable Holder.() -> Unit) {}

@Composable
fun lambdasValue(text: @Composable () -> Unit): Int = 1

@Composable
fun Lambdas10(vararg xs: Int, text: @Composable () -> Unit = {}) {}

val FooCompositionLocal = compositionLocalOf { 5 }
val LocalFoo = compositionLocalOf { 5 }
val localBar = staticCompositionLocalOf { 5 }
val TypedLocal: CompositionLocal<String?> = compositionLocalOf { null }
val Typed: ProvidableCompositionLocal<String?> = compositionLocalOf { null }
val Nullable: ProvidableCompositionLocal<String>? = null
val Lazy by lazy { compositionLocalOf { 1 } }
val Qualified = androidx.compose.runtime.compositionLocalOf { 1 }

object Locals {
    val BarCompositionLocal: CompositionLocal<String?> = staticCompositionLocalOf { null }
}

class WithCompanion {
    companion object {
        val BazCompositionLocal = compositionLocalOf { 1 }
    }
}

class Ctor(val CtorLocal: CompositionLocal<Int>)

fun usage(param: CompositionLocal<Int>) {
    val inside = compositionLocalOf { 1 }
}
