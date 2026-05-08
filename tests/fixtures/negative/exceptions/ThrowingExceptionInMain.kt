package exceptions

fun main(args: Array<String>) {
    if (args.isEmpty()) {
        println("No arguments provided")
        return
    }
    println("Hello, ${args[0]}")
}
