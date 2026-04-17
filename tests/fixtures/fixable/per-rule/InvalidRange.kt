package fixtures.positive.potentialbugs

class InvalidRange {
    fun example() {
        for (i in 10..1) {
            println(i)
        }
    }
}
