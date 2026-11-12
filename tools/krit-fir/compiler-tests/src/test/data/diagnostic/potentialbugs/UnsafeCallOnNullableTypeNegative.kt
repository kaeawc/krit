// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Operands and scopes that are not reported.
package test.unsafecall.negative

class Controller {
    fun start() {}
}

annotation class Preview

@Preview
fun detailPreview(name: String?): Int = name!!.length

fun sampleItem(name: String?): Int = name!!.length

fun fakeRepository(name: String?): Int = name!!.length

fun renderFixture(name: String?): Int = name!!.length

fun requireName(name: String?): String = name!!

fun stableSmartCast(name: String?): Int {
    if (name != null) {
        return name!!.length
    }
    return 0
}

class Registry {
    fun label(): String = Registry::class.simpleName!!

    fun key(map: Map<String, String>): String = map["key"]!!

    fun doubleKey(map: Map<String, Map<String, String>>): String = map["a"]!!["b"]!!
}

class Messages {
    fun text(value: String?): Int {
        val stringLiteral = "use !! to force"
        return value?.length ?: stringLiteral.length
    }
}
