package fixtures.positive.potentialbugs

class UnnamedParameterUse {
    fun configure(width: Int, height: Int, depth: Int, color: String, enabled: Boolean) {}

    fun example() {
        configure(100, 200, 300, "redColorValueForBackground", true)
    }
}
