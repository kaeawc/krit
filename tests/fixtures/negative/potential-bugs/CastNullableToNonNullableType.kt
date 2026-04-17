package potentialbugs

class Example {
    fun process(nonNull: String) {
        val result = nonNull as CharSequence
        println(result)
    }
}
