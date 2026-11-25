package p

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.padding
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.layout.LayoutModifier
import androidx.compose.ui.layout.ParentDataModifier
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp

// Good factories.
fun Modifier.good(): Modifier = this.then(Modifier.padding(1.dp))
fun Modifier.goodImplicit(): Modifier = padding(2.dp)
fun Modifier.goodExpression() = padding(3.dp)
fun Modifier.goodBlock(): Modifier {
    return background(Color.Red)
}
fun Modifier.goodThisChain(): Modifier = this.padding(4.dp).clip(RectangleShape)
fun Modifier.goodNested(): Modifier = then(Modifier.padding(5.dp))
fun Modifier.goodLabeled(): Modifier = run { this@goodLabeled.padding(1.dp) }
fun Modifier?.nullableReceiver(): Modifier = Modifier

// Not an extension.
fun noReceiver(): Modifier = Modifier.padding(1.dp)
fun noReceiverExpression() = Modifier.padding(1.dp)
fun Int.wrongReceiver(): Modifier = Modifier.padding(this.dp)

// Wrong return type.
fun Modifier.element(): Modifier.Element = object : Modifier.Element {}
fun Modifier.layoutModifier(): LayoutModifier = TODO()
fun Modifier.parentData(): ParentDataModifier = object : ParentDataModifier {
    override fun Density.modifyParentData(parentData: Any?): Any? = parentData
}
fun Modifier.nullableReturn(): Modifier? = this

// Unreferenced receiver.
fun Modifier.unreferenced(): Modifier = Modifier.padding(1.dp)
fun Modifier.unreferencedBlock(): Modifier {
    val other = Modifier.background(Color.Blue)
    return other
}
fun Modifier.unreferencedThen(): Modifier = Modifier.then(Modifier.padding(1.dp))
fun Modifier.unreferencedLambda(): Modifier = listOf(1).fold(Modifier as Modifier) { acc, _ -> acc.padding(1.dp) }

// Properties.
val Modifier.goodProperty: Modifier get() = padding(1.dp)
val Modifier.unreferencedProperty: Modifier get() = Modifier.padding(1.dp)
val noReceiverProperty: Modifier get() = Modifier.padding(1.dp)
val noReceiverInitializer: Modifier = Modifier.padding(1.dp)
var Modifier.mutableProperty: Modifier
    get() = Modifier
    set(value) {}
val Modifier.elementProperty: Modifier.Element get() = object : Modifier.Element {}

// Members.
class Holder {
    fun member(): Modifier = Modifier.padding(1.dp)
    fun Modifier.memberExtension(): Modifier = padding(1.dp)
    val memberProperty: Modifier get() = Modifier
}

interface Factory {
    fun create(): Modifier
}

abstract class Base {
    abstract fun Modifier.abstractExtension(): Modifier
}

object Impl : Factory {
    override fun create(): Modifier = Modifier
}

fun outer() {
    fun localFactory(): Modifier = Modifier
}

fun notAModifier(): Int = 1
fun Modifier.returnsUnit() {}
