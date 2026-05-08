package complexity

class CognitiveComplexMethod {
    fun deeplyNested(x: Int, y: Int, z: Int, list: List<Int>): String {
        if (x > 0) {                              // +1
            for (i in list) {                      // +1 +1(nesting)
                if (i > 0) {                       // +1 +2(nesting)
                    when (y) {                     // (when_expression nests)
                        1 -> println("one")        // +1 +3(nesting)
                        2 -> println("two")        // +1 +3(nesting)
                        else -> {
                            if (z > 0) {           // +1 +4(nesting)
                                println("deep")
                            }
                        }
                    }
                }
            }
        }
        if (y > 0) {                              // +1
            if (z > 0) {                           // +1 +1(nesting)
                for (j in list) {                  // +1 +2(nesting)
                    if (j < 0) {                   // +1 +3(nesting)
                        println("neg")
                    }
                }
            }
        }
        if (z < 0) {                              // +1
            println("negative z")
        }
        return "done"
    }
}
