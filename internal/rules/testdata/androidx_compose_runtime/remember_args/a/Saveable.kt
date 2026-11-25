package a

import androidx.compose.runtime.Composable
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.Saver
import androidx.compose.runtime.saveable.listSaver
import androidx.compose.runtime.saveable.rememberSaveable

class Foo(val x: Int)

object FooSaver : Saver<Foo, Int> {
    override fun androidx.compose.runtime.saveable.SaverScope.save(value: Foo): Int = value.x
    override fun restore(value: Int): Foo = Foo(value)
}

class FooSaverClass : Saver<Foo, Int> {
    override fun androidx.compose.runtime.saveable.SaverScope.save(value: Foo): Int = value.x
    override fun restore(value: Int): Foo = Foo(value)
}

val fooListSaver = listSaver<Foo, Int>(save = { listOf(it.x) }, restore = { Foo(it[0]) })

@Composable
fun Saveables() {
    val a = rememberSaveable(FooSaver) { Foo(1) }
    val b = rememberSaveable(FooSaverClass()) { Foo(1) }
    val c = rememberSaveable(fooListSaver) { Foo(1) }
    val d = rememberSaveable(FooSaver) { mutableStateOf(Foo(1)) }
    val e = rememberSaveable(saver = FooSaver) { Foo(1) }
    val f = rememberSaveable(stateSaver = FooSaver) { mutableStateOf(Foo(1)) }
    val g = rememberSaveable(Any(), FooSaver, Any()) { Foo(1) }
    val h = rememberSaveable(Saver<Foo, Int>(save = { it.x }, restore = { Foo(it) })) { Foo(1) }
    val i = rememberSaveable(1) { Foo(1) }
}
