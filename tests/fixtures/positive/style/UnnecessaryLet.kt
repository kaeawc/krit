package style

class Widget {
    fun render(): Int = 1
}

fun example(nullable: Widget?, widget: Widget) {
    nullable?.let { println("checked") }
    widget.let { it.render() }
    val copy = widget.let { it }
}
