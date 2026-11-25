package p.nodes

class ViaOtherFile : OtherFileBase() {
    override fun create() = MyNode()
    override fun update(node: MyNode) {}
    override fun hashCode() = 0
    override fun equals(other: Any?) = false
}
