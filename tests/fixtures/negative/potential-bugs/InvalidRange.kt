package fixtures.negative.potentialbugs

class InvalidRange {
    fun example() {
        for (i in 1..10) {
            println(i)
        }
    }
}
