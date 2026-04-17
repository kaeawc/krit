package fixtures.negative.potentialbugs

class UnnamedParameterUse {
    fun setSize(width: Int, height: Int) {}

    fun example() {
        setSize(width = 100, height = 200)
    }
}
