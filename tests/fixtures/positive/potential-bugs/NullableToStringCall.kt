package potentialbugs

class Example {
    fun format(value: String?) {
        val text = value?.toString()
        println(text)
    }
}
