package style

class Widget {
    fun render(): Int = 1
}

fun example(nullable: Widget?, widget: Widget, items: List<Int>) {
    nullable?.let {
        println(it.render())
        println(it)
    }
    val size = items.let { it.size + it.sum() }
    val nested = widget.copyOf().let { println(it) }
    val total = nullable?.let { println(it) }
}

fun Widget.copyOf(): Widget = this
