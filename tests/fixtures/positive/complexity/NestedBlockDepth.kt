package complexity

class NestedBlockDepth {
    fun deeplyNested(a: Int, b: Int, c: Int, d: Int, e: Int) {
        if (a > 0) {
            if (b > 0) {
                if (c > 0) {
                    if (d > 0) {
                        if (e > 0) {
                            println("deep")
                        }
                    }
                }
            }
        }
    }
}
