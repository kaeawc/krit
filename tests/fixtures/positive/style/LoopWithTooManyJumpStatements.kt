package style

fun example(items: List<Int>) {
    for (item in items) {
        if (item < 0) break
        if (item > 100) break
        if (item == 50) continue
        if (item == 25) break
        println(item)
    }
}
