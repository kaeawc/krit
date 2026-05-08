package complexity

class CyclomaticComplexMethod {
    fun complexFunction(x: Int, y: Int, z: Int, list: List<Int>): String {
        // 15+ decision points: if/else if/when/for/&&/||
        if (x > 0) {                    // 1
            println("a")
        } else if (x < -10) {           // 2
            println("b")
        }
        if (y > 0) {                    // 3
            println("c")
        }
        if (z > 0) {                    // 4
            println("d")
        }
        if (x > 0 && y > 0) {           // 5 (if) + 6 (&&)
            println("e")
        }
        if (x > 0 || z < 0) {           // 7 (if) + 8 (||)
            println("f")
        }
        for (i in list) {               // 9
            if (i > 0) {                // 10
                println("g")
            }
        }
        if (y < 0 && z > 0) {           // 11 (if) + 12 (&&)
            println("h")
        }
        if (z < 0 || y > 100) {         // 13 (if) + 14 (||)
            println("i")
        }
        if (list.size > 5) {            // 15
            println("j")
        }
        return "done"
    }
}
