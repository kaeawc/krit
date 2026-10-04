package style

fun example() {
    // The element type is declared so the fixture compiles for FIR parity: a
    // bare `listOf()` cannot infer its type parameter.
    val list: List<String> = listOf()
}
