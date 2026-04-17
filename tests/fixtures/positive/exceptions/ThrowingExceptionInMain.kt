package exceptions

fun main(args: Array<String>) {
    if (args.isEmpty()) {
        throw IllegalArgumentException("No arguments provided")
    }
    println("Hello, ${args[0]}")
}
