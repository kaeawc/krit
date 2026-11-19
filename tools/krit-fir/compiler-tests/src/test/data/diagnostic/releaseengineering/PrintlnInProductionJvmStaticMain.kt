// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 27, 32
// A @JvmStatic main of an object or companion object is a JVM entry point, so
// it may print like a top-level main. Without @JvmStatic, with another
// signature, or in a class it is not an entry point and still reports.
package test

object Cli {
    @JvmStatic
    fun main(args: Array<String>) {
        println("entry point")
        listOf(1).forEach { System.out.println(it) }
    }
}

class App {
    companion object {
        @JvmStatic fun main(vararg args: String) { println("entry point") }
    }
}

object NotEntry {
    fun main(args: Array<String>) { <!PrintlnInProduction!>println<!>("no JvmStatic") }
}

object WrongSignature {
    @JvmStatic fun main(a: Int) { <!PrintlnInProduction!>println<!>("not a main signature") }
}

class Member {
    companion object {
        @JvmStatic private fun main(args: Array<String>) { <!PrintlnInProduction!>println<!>("private") }
    }
}
