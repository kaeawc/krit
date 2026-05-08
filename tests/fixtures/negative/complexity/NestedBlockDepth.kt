package complexity

class NestedBlockDepth {
    fun shallowNested(a: Int, b: Int, c: Int) {
        if (a > 0) {
            if (b > 0) {
                if (c > 0) {
                    println("ok")
                }
            }
        }
    }
}
