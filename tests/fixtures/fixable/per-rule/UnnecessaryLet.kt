package style

class Widget {
    fun render(): Int = 1
}

fun example(widget: Widget, nullable: Widget?) {
    val first = widget.let { it.render() }
    val second = widget.let { it }
    val third = nullable?.let { it.render() }
}
