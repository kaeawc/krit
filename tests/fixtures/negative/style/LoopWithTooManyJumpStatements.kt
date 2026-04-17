package style

fun example(items: List<Int>) {
    for (item in items) {
        if (item < 0) break
        println(item)
    }
}
