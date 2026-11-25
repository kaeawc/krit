package p.nodes

import androidx.compose.ui.Modifier
import androidx.compose.ui.node.ModifierNodeElement
import androidx.compose.ui.platform.InspectorInfo

class MyNode : Modifier.Node()

class Missing : ModifierNodeElement<MyNode>() {
    override fun create() = MyNode()
    override fun update(node: MyNode) {}
    override fun hashCode() = 0
    override fun equals(other: Any?) = other is Missing
}

data class Present(val x: Int) : ModifierNodeElement<MyNode>() {
    override fun create() = MyNode()
    override fun update(node: MyNode) {}
    override fun InspectorInfo.inspectableProperties() {
        name = "present"
    }
}

abstract class BaseElement : ModifierNodeElement<MyNode>() {
    override fun InspectorInfo.inspectableProperties() {}
}

class Inherited : BaseElement() {
    override fun create() = MyNode()
    override fun update(node: MyNode) {}
    override fun hashCode() = 0
    override fun equals(other: Any?) = other is Inherited
}

abstract class AbstractMissing : ModifierNodeElement<MyNode>()

class ViaAbstract : AbstractMissing() {
    override fun create() = MyNode()
    override fun update(node: MyNode) {}
    override fun hashCode() = 0
    override fun equals(other: Any?) = other is ViaAbstract
}

class WrongSignature : ModifierNodeElement<MyNode>() {
    override fun create() = MyNode()
    override fun update(node: MyNode) {}
    override fun hashCode() = 0
    override fun equals(other: Any?) = other is WrongSignature
    fun inspectableProperties() {}
}

val anonymous = object : ModifierNodeElement<MyNode>() {
    override fun create() = MyNode()
    override fun update(node: MyNode) {}
    override fun hashCode() = 0
    override fun equals(other: Any?) = false
}
