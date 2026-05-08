package potentialbugs

class Example {
    fun firstPositive(list: List<Int>): Int {
        for (x in list) {
            if (x > 0) return x
        }
        return -1
    }
}
