package potentialbugs

data class Box<T>(val value: T)

class Example {
    fun nullableDeclaration(input: String?) {
        val result = input as String
        println(result)
    }

    fun inferredNullable(flag: Boolean): String {
        val value = if (flag) "ok" else null
        return value as String
    }

    fun genericCast(input: Box<String>?) {
        val result = input as Box<String>
        println(result)
    }

    fun multilineCast(input: String?) {
        val result =
            (input) as // keep expression multiline
                String
        println(result)
    }
}
