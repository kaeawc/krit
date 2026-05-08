package fixtures.negative.potentialbugs

class LateinitUsage {
    var name: String = ""

    fun init() {
        name = "hello"
    }
}
