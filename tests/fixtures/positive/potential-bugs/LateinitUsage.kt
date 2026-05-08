package fixtures.positive.potentialbugs

class LateinitUsage {
    lateinit var name: String

    fun init() {
        name = "hello"
    }
}
