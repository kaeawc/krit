package potentialbugs

class Example {
    fun firstElement(list: List<Int>): Int {
        for (x in list) {
            return x
        }
        return -1
    }
}
