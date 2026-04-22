package potentialbugs

data class Box<T>(val value: T)
typealias NullableName = String?

class Example {
    fun safeCast(input: Any?) {
        val result = input as? String
        println(result)
    }

    fun nullableTarget(input: String?) {
        val result = input as String?
        println(result)
    }

    fun nullLiteral() {
        val result = null as String
        println(result)
    }

    fun unresolvedSource() {
        val result = missing as String
        println(result)
    }

    fun nonNullSource(nonNull: String) {
        val result = nonNull as CharSequence
        println(result)
    }

    fun aliasTarget(input: String?) {
        val result = input as NullableName
        println(result)
    }

    fun commentsAndStrings(input: String?) {
        // input as String
        val text = "input as String"
        println(text)
        val safe = input as? String
        println(safe)
    }

    fun shadowing(input: String?) {
        run {
            val input: String = "local"
            val result = input as CharSequence
            println(result)
        }
    }
}
